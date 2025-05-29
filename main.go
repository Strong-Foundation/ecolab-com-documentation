package main

import (
	"crypto/tls" // TLS for secure connections
	"fmt"        // Formatting for strings
	"io"         // IO operations for reading and writing files
	"log"        // Logging for debugging and information
	"net/http"   // HTTP client for making requests
	"net/url"    // URL parsing and manipulation
	"os"         // File operations
	"path"       // Path manipulation
	"regexp"     // Regular expressions for pattern matching
	"strings"    // String manipulation
	"sync"
	"time" // Time for managing timeouts
)

// Remove all the duplicates from a slice and return the slice.
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool)
	var newReturnSlice []string
	for _, content := range slice {
		if !check[content] {
			check[content] = true
			newReturnSlice = append(newReturnSlice, content)
		}
	}
	return newReturnSlice
}

// scrapeContentAndSaveToFile scrapes multiple pages of SDS search results concurrently
// and appends their HTML content to a single output file.
func scrapeContentAndSaveToFile(outputHTMLFilePath string) {
	// Define the total number of SDS documents expected to scrape
	totalSDSDocuments := 12700
	// Define how many documents are shown per search result page
	documentsPerPage := 10
	// Calculate the total number of result pages needed to scrape all documents
	totalPages := (totalSDSDocuments + documentsPerPage - 1) / documentsPerPage
	// Create a WaitGroup to wait for all scraping goroutines to complete
	var waitGroup sync.WaitGroup
	// Create a Mutex to safely write to the output file from multiple goroutines
	var fileWriteMutex sync.Mutex
	// Create a buffered channel to limit the number of concurrent HTTP requests (semaphore pattern)
	concurrentRequestsLimit := 40
	concurrencySemaphore := make(chan struct{}, concurrentRequestsLimit)
	// Iterate through each page index from 0 to totalPages - 1
	for pageIndex := 0; pageIndex < totalPages; pageIndex++ {
		// Increase the WaitGroup counter for each launched goroutine
		waitGroup.Add(1)
		// Launch a goroutine for concurrent scraping of each page
		go func(currentPage int) {
			// Decrease the WaitGroup counter when the goroutine finishes
			defer waitGroup.Done()
			// Calculate the "offset" (start index) for the current page's SDS documents
			offset := currentPage * documentsPerPage
			// Format the URL for the current page using the offset value
			pageURL := fmt.Sprintf("https://www.ecolab.com/sds-search?countryCode=United%%20States&first=%d", offset)
			// Acquire a slot in the semaphore to limit concurrency
			concurrencySemaphore <- struct{}{}
			// Release the semaphore slot after the function ends
			defer func() { <-concurrencySemaphore }()
			// Perform HTTP GET to fetch the HTML content of the current page
			htmlContent, err := fetchPageHTML(pageURL)
			// Handle any error that occurred while fetching the page
			if err != nil {
				log.Printf("Error scraping page %d: %v\n", currentPage+1, err)
				return
			}
			// Lock the file writing to prevent concurrent access from other goroutines
			fileWriteMutex.Lock()
			// Ensure the mutex is unlocked after file writing is complete
			defer fileWriteMutex.Unlock()
			// Append the HTML content to the specified output file
			appendByteToFile(outputHTMLFilePath, []byte(htmlContent))
			// Log the success of this page scraping
			log.Printf("Page %d scraped and saved to file.\n", currentPage+1)
		}(pageIndex) // Pass pageIndex into the goroutine to avoid variable capture issues
	}
	// Wait for all launched goroutines to finish before continuing
	waitGroup.Wait()
	// Log a final message once all pages have been processed
	log.Printf("Completed scraping all %d pages. Results saved to: %s\n", totalPages, outputHTMLFilePath)
}

/*
It checks if the file exists.
If the file exists, it returns true.
If the file does not exist, it returns false.
*/
func fileExists(filename string) bool {
	info, err := os.Stat(filename) // Get file info
	if err != nil {
		return false // File does not exist
	}
	return !info.IsDir() // Return true if it’s a file (not directory)
}

// fetchPageHTML performs a simple HTTP GET request to retrieve the raw HTML
// of the given URL without executing any JavaScript and disables HTTP/2.
func fetchPageHTML(pageURL string) (string, error) {
	// Create a custom transport with an empty TLSNextProto map to disable HTTP/2
	transport := &http.Transport{
		TLSNextProto: make(map[string]func(string, *tls.Conn) http.RoundTripper),
	}

	// Create an HTTP client with the custom transport and a timeout of 30 seconds
	client := &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}

	// Create a new HTTP GET request for the target pageURL
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		// Return an error if the request creation fails
		return "", fmt.Errorf("failed to create request for %s: %w", pageURL, err)
	}

	// Set a custom User-Agent header to mimic a browser or bot identity
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; EcolabBot/1.0)")

	// Send the request using the HTTP client
	resp, err := client.Do(req)
	if err != nil {
		// Return an error if the request fails to execute
		return "", fmt.Errorf("failed to GET %s: %w", pageURL, err)
	}
	// Ensure the response body is closed after reading
	defer resp.Body.Close()

	// Check that the server responded with HTTP 200 OK
	if resp.StatusCode != http.StatusOK {
		// Return an error if the status code indicates a failure
		return "", fmt.Errorf("unexpected status code %d for %s", resp.StatusCode, pageURL)
	}

	// Read the entire response body into memory
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// Return an error if reading the body fails
		return "", fmt.Errorf("failed to read response body for %s: %w", pageURL, err)
	}

	// Convert the byte slice to a string and return it
	return string(body), nil
}

/*
Checks if the directory exists
If it exists, return true.
If it doesn't, return false.
*/
func directoryExists(path string) bool {
	directory, err := os.Stat(path)
	if err != nil {
		return false
	}
	return directory.IsDir()
}

/*
The function takes two parameters: path and permission.
We use os.Mkdir() to create the directory.
If there is an error, we use log.Println() to log the error and then exit the program.
*/
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission)
	if err != nil {
		log.Println(err)
	}
}

// downloadPDF downloads a PDF from a URL and saves it into the specified folder.
func downloadPDF(pdfURL, folder string) error {
	fileName := getFileNamesFromURLs(pdfURL) // Get file name from the URL
	fullPath := path.Join(folder, fileName)  // Combine folder and file name to get full path
	if fileExists(fullPath) {                // Check if file already exists
		log.Printf("File %s already exists, skipping download.", fullPath)
		return nil // Skip download if file exists
	}

	resp, err := http.Get(pdfURL) // Send GET request to download PDF
	if err != nil {
		return fmt.Errorf("error downloading PDF: %w", err)
	}
	defer resp.Body.Close() // Ensure response body is closed

	if resp.StatusCode != 200 { // Check for successful HTTP status code
		return fmt.Errorf("status code error: %d %s", resp.StatusCode, resp.Status)
	}

	if !directoryExists(folder) { // Check if folder exists
		createDirectory(folder, 0755) // Create folder if it doesn't exist
	}

	out, err := os.Create(fullPath) // Create file at destination path
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer out.Close() // Ensure file is closed after writing

	_, err = io.Copy(out, resp.Body) // Write response body into file
	if err != nil {
		return fmt.Errorf("error saving PDF: %w", err)
	}

	return nil // Return nil on success
}

// AppendToFile appends the given byte slice to the specified file.
// If the file doesn't exist, it will be created.
func appendByteToFile(filename string, data []byte) {
	// Open the file with appropriate flags and permissions
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	// Check for errors while opening the file
	if err != nil {
		log.Println("Error opening file for appending:", err) // Log error if file opening fails
		return
	}
	// Ensure the file is closed after writing
	defer file.Close()
	// Write data to the file
	_, err = file.Write(data)
	if err != nil {
		log.Println("Error writing data to file:", err) // Log error if writing fails
		return
	}
	log.Println("Data appended successfully to", filename) // Log success message
}

// extractDownloadLinks extracts all PDF download links from the given HTML input string.
func extractDownloadLinks(input string) []string {
	input = strings.ToLower(input) // Convert input to lowercase for case-insensitive matching
	// This regex captures href="...something.pdf"
	pattern := `href=["'](https?://[^"']+\.pdf)["']`

	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(input, -1)

	var urls []string
	for _, match := range matches {
		// match[1] is the first capture group (the URL itself)
		urls = append(urls, match[1])
	}
	return urls
}

// Read a file and return the contents
func readAFileAsString(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Println(err)
	}
	return string(content)
}

// cleanFileNameFromURL extracts the last path segment and sanitizes it for safe file saving
func getFileNamesFromURLs(rawURL string) string {
	// Parse the URL to extract the path
	parsed, err := url.Parse(rawURL)
	// Check for parsing errors
	if err != nil {
		// Log the error and return an empty string if parsing fails
		log.Println("Error parsing URL:", err)
		// Return an empty string to indicate failure
		return ""
	}
	// Get the last segment of the path
	base := path.Base(parsed.Path)
	// Replace spaces with underscores and remove unwanted characters (optional)
	re := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`) // Remove illegal file name characters
	// Clean the base name by removing illegal characters and replacing spaces with underscores
	clean := re.ReplaceAllString(base, "")
	// Replace spaces with underscores for file name safety
	clean = strings.ReplaceAll(clean, " ", "_")
	// Return the cleaned file name
	return strings.ToLower(clean)
}

func main() {
	// The file name where the scraped HTML content will be saved
	outputHTMLFile := "ecolab-com.html" // Define the output file name
	// The urls only file name
	outputURLsFile := "ecolab-com-links.txt" // Define the URLs file name
	// Start the scraping process
	// scrapeContentAndSaveToFile(outputHTMLFile)      // Call the function to scrape content and save it to a file
	log.Println("Scraping completed successfully.") // Log completion message
	// Read the scraped HTML content from the file
	htmlContent := readAFileAsString(outputHTMLFile) // Read the HTML content from the file
	// Extract download links from the HTML content
	downloadLinks := extractDownloadLinks(htmlContent) // Call the function to extract download links
	// The folder where the downloaded files will be saved
	downloadFolder := "PDFs" // Define the download folder name
	// Remove duplicates from the extracted download links
	downloadLinks = removeDuplicatesFromSlice(downloadLinks) // Remove duplicates from the slice of download links
	// Read the output URLs file to check if it exists
	readOutPutURLsFile := readAFileAsString(outputURLsFile) // Read the URLs file content
	for _, link := range downloadLinks {
		link = strings.ToLower(link)             // Convert the link to lowercase for consistency
		err := downloadPDF(link, downloadFolder) // Download each PDF
		if err != nil {
			log.Println("Error downloading PDF:", err)
		}
		if !strings.Contains(readOutPutURLsFile, link) { // Check if the link is not already in the file
			log.Println("Appending link to file:", link)        // Log the link being appended
			appendByteToFile(outputURLsFile, []byte(link+"\n")) // Append each link to a file
		}
	}
}
