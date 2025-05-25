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
	"time"       // Time for managing timeouts

	"golang.org/x/net/html" // HTML parsing and manipulation
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

// scrapeContentAndSaveToFile scrapes multiple pages of content and saves the HTML to a file.
func scrapeContentAndSaveToFile(outputHTMLFile string) {
	// Define the total number of documents to scrape and how many per page
	totalDocumentsToScrape := 12700
	// Define how many documents to scrape per page and calculate total pages
	documentsPerPage := 10
	// Calculate total pages based on total documents and documents per page
	totalPages := (totalDocumentsToScrape + documentsPerPage - 1) / documentsPerPage
	for currentPageIndex := 0; currentPageIndex < totalPages; currentPageIndex++ {
		// Calculate the offset for the current page based on the index and documents per page
		offset := currentPageIndex * documentsPerPage
		// Correctly format the URL
		pageURL := fmt.Sprintf("https://www.ecolab.com/sds-search?countryCode=United%%20States&first=%d", offset)
		// Log the URL being scraped
		var pageHTMLContent string
		// Call the scraping function to get the HTML content of the page
		pageHTMLContent, err := fetchPageHTML(pageURL)
		// Check for errors during scraping
		if err != nil {
			log.Printf("Error scraping page %d: %v\n", currentPageIndex+1, err)
			continue
		}
		// Log the successful scraping of the page
		appendByteToFile(outputHTMLFile, []byte(pageHTMLContent))
		// Log the successful append operation
		log.Printf("Page %d scraped successfully and appended to %s\n", currentPageIndex+1, outputHTMLFile)
	}
	log.Printf("All scraped content has been saved to %s\n", outputHTMLFile)
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
		Timeout:   30 * time.Second,
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
If there is an error, we use log.Fatalln() to log the error and then exit the program.
*/
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission)
	if err != nil {
		log.Fatalln(err)
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

// ExtractDownloadLinks parses the HTML and returns all .pdf hrefs from <a class="sds-downloadBtn">
func ExtractDownloadLinks(htmlContent string) ([]string, error) {
	var links []string

	tokenizer := html.NewTokenizer(strings.NewReader(htmlContent))

	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			return links, nil // End of document

		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if token.Data == "a" {
				var href string
				var isDownloadBtn bool

				for _, attr := range token.Attr {
					if attr.Key == "class" && strings.Contains(attr.Val, "sds-downloadBtn") {
						isDownloadBtn = true
					}
					if attr.Key == "href" {
						href = attr.Val
					}
				}

				if isDownloadBtn && strings.HasSuffix(strings.ToLower(href), ".pdf") {
					links = append(links, href)
				}
			}
		}
	}
}

// Read a file and return the contents
func readAFileAsString(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Fatalln(err)
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
	// Start the scraping process
	// scrapeContentAndSaveToFile(outputHTMLFile)      // Call the function to scrape content and save it to a file
	log.Println("Scraping completed successfully.") // Log completion message
	// Read the scraped HTML content from the file
	htmlContent := readAFileAsString(outputHTMLFile) // Read the HTML content from the file
	// Extract download links from the HTML content
	downloadLinks, err := ExtractDownloadLinks(htmlContent) // Call the function to extract download links
	if err != nil {
		log.Fatalln("Error extracting download links:", err) // Log error if extraction fails
	}
	// The folder where the downloaded files will be saved
	downloadFolder := "PDFs" // Define the download folder name
	// Remove duplicates from the extracted download links
	downloadLinks = removeDuplicatesFromSlice(downloadLinks) // Remove duplicates from the slice of download links
	for _, link := range downloadLinks {
		err := downloadPDF(link, downloadFolder) // Download each PDF
		if err != nil {
			log.Println("Error downloading PDF:", err)
		}
		// appendByteToFile("ecolab-com-links.txt", []byte(link+"\n")) // Append each link to a file
	}
}
