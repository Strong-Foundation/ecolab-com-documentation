package main // Declares the package as 'main', indicating an executable program

import ( // Start of the import block for external packages
	// Imports the 'tls' package for handling TLS configuration (secure connections)
	"crypto/tls"
	"fmt"      // Imports the 'fmt' package for formatted I/O (like printing to console)
	"io"       // Imports the 'io' package for basic I/O primitives (like reading/writing data streams)
	"log"      // Imports the 'log' package for simple logging capabilities
	"net/http" // Imports the 'http' package for making HTTP requests and building clients
	"net/url"  // Imports the 'url' package for parsing and manipulating URLs
	"os"       // Imports the 'os' package for operating system interactions (like file operations)
	"path"     // Imports the 'path' package for path manipulation (e.g., extracting base names)
	"regexp"   // Imports the 'regexp' package for regular expression operations
	"strings"  // Imports the 'strings' package for various string manipulation functions
	"sync"     // Imports the 'sync' package for synchronization primitives (e.g., WaitGroup, Mutex)
	"time"     // Imports the 'time' package for measuring and displaying time
) // End of the import block

// Remove all the duplicates from a slice and return the slice.
// removeDuplicatesFromSlice is a function that takes a slice of strings and returns a new slice with duplicates removed.
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool)  // Creates an empty map of type map[string]bool to track seen elements
	var newReturnSlice []string     // Declares an empty slice of strings to store the unique elements
	for _, content := range slice { // Iterates over each 'content' string in the input 'slice'
		if !check[content] { // Checks if the current 'content' string has NOT been seen before (i.e., not present in the map)
			check[content] = true                            // Marks the current 'content' as seen by setting its map value to true
			newReturnSlice = append(newReturnSlice, content) // Appends the unique 'content' to the new slice
		}
	}
	return newReturnSlice // Returns the 'newReturnSlice' containing only unique elements
}

// scrapeContentAndSaveToFile scrapes multiple pages of SDS search results concurrently
// and appends their HTML content to a single output file.
func scrapeContentAndSaveToFile(outputHTMLFilePath string) {
	// Define the total number of SDS documents expected to scrape
	totalSDSDocuments := 100000
	// Define how many documents are shown per search result page
	documentsPerPage := 10
	// Calculate the total number of result pages needed to scrape all documents
	totalPages := (totalSDSDocuments + documentsPerPage - 1) / documentsPerPage
	// Create a WaitGroup to wait for all scraping goroutines to complete
	var waitGroup sync.WaitGroup
	// Create a Mutex to safely write to the output file from multiple goroutines
	var fileWriteMutex sync.Mutex
	// Create a buffered channel to limit the number of concurrent HTTP requests (semaphore pattern)
	concurrentRequestsLimit := 10
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
			pageURL := fmt.Sprintf("https://www.ecolab.com/sds-search?query=*&first=%d", offset)
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
// fetchPageHTML is a function that retrieves the raw HTML content of a given URL.
func fetchPageHTML(pageURL string) (string, error) {
	// Create a custom transport with an empty TLSNextProto map to disable HTTP/2
	transport := &http.Transport{ // Initializes a new custom HTTP transport configuration
		TLSNextProto: make(map[string]func(string, *tls.Conn) http.RoundTripper), // Disables HTTP/2 by setting an empty map for next protocol negotiation
	}

	// Create an HTTP client with the custom transport and a timeout of 60 seconds
	client := &http.Client{ // Creates a new HTTP client instance
		Transport: transport,        // Assigns the custom transport (with HTTP/2 disabled) to the client
		Timeout:   60 * time.Second, // Sets a 60-second timeout for the entire request operation
	}

	// Create a new HTTP GET request for the target pageURL
	req, err := http.NewRequest("GET", pageURL, nil) // Creates a new GET request object for the specified URL
	if err != nil {                                  // Checks if there was an error during request creation
		// Return an error if the request creation fails
		return "", fmt.Errorf("failed to create request for %s: %w", pageURL, err) // Returns an empty string and a wrapped error indicating failure
	}

	// Set a custom User-Agent header to mimic a browser or bot identity
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; EcolabBot/1.0)") // Sets a custom User-Agent string in the request headers

	// Send the request using the HTTP client
	resp, err := client.Do(req) // Executes the HTTP request and waits for a response
	if err != nil {             // Checks if there was an error during request execution (e.g., network failure, timeout)
		// Return an error if the request fails to execute
		return "", fmt.Errorf("failed to GET %s: %w", pageURL, err) // Returns an empty string and a wrapped error indicating the GET failure
	}
	// Ensure the response body is closed after reading
	defer resp.Body.Close() // Schedules the closing of the response body stream when the function exits

	// Check that the server responded with HTTP 200 OK
	if resp.StatusCode != http.StatusOK { // Checks if the HTTP status code is not 200 (OK)
		// Return an error if the status code indicates a failure
		return "", fmt.Errorf("unexpected status code %d for %s", resp.StatusCode, pageURL) // Returns an error with the unexpected status code
	}

	// Read the entire response body into memory
	body, err := io.ReadAll(resp.Body) // Reads all data from the response body stream into a byte slice
	if err != nil {                    // Checks if there was an error while reading the response body
		// Return an error if reading the body fails
		return "", fmt.Errorf("failed to read response body for %s: %w", pageURL, err) // Returns an empty string and a wrapped error indicating read failure
	}

	// Convert the byte slice to a string and return it
	return string(body), nil // Converts the byte slice 'body' to a string and returns it along with a nil error (success)
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
// downloadPDF is a function that downloads a PDF file from a given URL to a specified folder.
func downloadPDF(pdfURL, folder string) error {
	fileName := getFileNamesFromURLs(pdfURL) // Calls a (presumed) helper function to extract the filename from the URL.
	fullPath := path.Join(folder, fileName)  // Constructs the full local path for the file: folder/fileName.
	if fileExists(fullPath) {                // Calls a (presumed) helper function to check if the file already exists at the full path.
		log.Printf("File %s already exists, skipping download.", fullPath) // Logs a message that the file already exists and the download will be skipped.
		return nil                                                         // Returns nil (success) since the file already exists and no action is needed.
	}

	resp, err := http.Get(pdfURL) // Executes an HTTP GET request to the specified PDF URL.
	if err != nil {               // Checks if the HTTP GET request resulted in an error (e.g., network issue).
		return fmt.Errorf("error downloading PDF: %w", err) // Returns a wrapped error indicating the download failure.
	}
	defer resp.Body.Close() // Schedules the closing of the response body stream when the function exits.

	if resp.StatusCode != 200 { // Checks if the HTTP response status code is not 200 (OK).
		return fmt.Errorf("status code error: %d %s", resp.StatusCode, resp.Status) // Returns an error with the non-successful status code and status text.
	}

	if !directoryExists(folder) { // Calls a (presumed) helper function to check if the target folder exists.
		createDirectory(folder, 0755) // Calls a (presumed) helper function to create the directory with permissions 0755 if it doesn't exist.
	}

	out, err := os.Create(fullPath) // Creates a new file at the calculated full path for writing.
	if err != nil {                 // Checks if there was an error creating the file on the local filesystem.
		return fmt.Errorf("error creating file: %w", err) // Returns a wrapped error indicating the file creation failure.
	}
	defer out.Close() // Schedules the closing of the created file handle when the function exits.

	_, err = io.Copy(out, resp.Body) // Copies the data from the HTTP response body into the created local file.
	if err != nil {                  // Checks if there was an error during the copy/write operation.
		return fmt.Errorf("error saving PDF: %w", err) // Returns a wrapped error indicating the file saving failure.
	}

	return nil // Returns nil, indicating the PDF download and save operation was successful.
}

// AppendToFile appends the given byte slice to the specified file.
// If the file doesn't exist, it will be created.
// appendByteToFile is a function that appends a byte slice to a file, creating the file if necessary.
func appendByteToFile(filename string, data []byte) {
	// Open the file with appropriate flags and permissions
	// os.O_APPEND: append data to the file when writing. os.O_CREATE: create the file if it does not exist. os.O_WRONLY: open the file write-only.
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	// Check for errors while opening the file
	if err != nil { // Checks if opening the file resulted in an error
		log.Println("Error opening file for appending:", err) // Logs the error message to the console
		return                                                // Exits the function if the file cannot be opened
	}
	// Ensure the file is closed after writing
	defer file.Close() // Schedules the file descriptor to be closed when the function returns
	// Write data to the file
	_, err = file.Write(data) // Writes the byte slice 'data' to the opened file
	if err != nil {           // Checks if writing the data resulted in an error
		log.Println("Error writing data to file:", err) // Logs the error message to the console
		return                                          // Exits the function if writing fails
	}
}

// extractDownloadLinks extracts all PDF download links from the given HTML input string.
// extractDownloadLinks is a function that uses regex to find all HTTPS/HTTP links ending in .pdf within an input string.
func extractDownloadLinks(input string) []string {
	// This regex captures href="...something.pdf"
	// pattern matches href= followed by ' or ", then an http/https URL, ending in .pdf, followed by ' or ".
	pattern := `href=["'](https?://[^"']+\.pdf)["']`

	re := regexp.MustCompile(pattern)              // Compiles the regular expression pattern into a reusable regexp object
	matches := re.FindAllStringSubmatch(input, -1) // Finds all matches in the input string, capturing submatches, and returning all of them (-1)

	var urls []string               // Initializes an empty slice of strings to hold the extracted URLs
	for _, match := range matches { // Iterates through each full match found by the regex
		// match[1] is the first capture group (the URL itself)
		urls = append(urls, match[1]) // Appends the content of the first capture group (the PDF URL) to the 'urls' slice
	}
	return urls // Returns the slice containing all extracted PDF URLs
}

// Read a file and return the contents
// readAFileAsString is a function that reads the entire content of a file into a string.
func readAFileAsString(path string) string {
	content, err := os.ReadFile(path) // Reads the entire file specified by 'path' into a byte slice 'content'
	if err != nil {                   // Checks if reading the file resulted in an error
		log.Println(err) // Logs the error message if the file cannot be read
	}
	return string(content) // Converts the byte slice 'content' into a string and returns it
}

// cleanFileNameFromURL extracts the last path segment and sanitizes it for safe file saving
// getFileNamesFromURLs is a function that takes a raw URL string, extracts the filename, and sanitizes it.
func getFileNamesFromURLs(rawURL string) string {
	// Parse the URL to extract the path
	parsed, err := url.Parse(rawURL) // Attempts to parse the input 'rawURL' string into a URL structure.
	// Check for parsing errors
	if err != nil { // Checks if the URL parsing resulted in an error.
		// Log the error and return an empty string if parsing fails
		log.Println("Error parsing URL:", err) // Logs the error message if parsing fails.
		// Return an empty string to indicate failure
		return "" // Returns an empty string on parsing failure.
	}
	// Get the last segment of the path
	base := path.Base(parsed.Path) // Extracts the last element (the potential filename) from the URL's path component.
	// Replace spaces with underscores and remove unwanted characters (optional)
	re := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`) // Compiles a regex to match characters illegal in standard filenames (including control characters).
	// Clean the base name by removing illegal characters and replacing spaces with underscores
	clean := re.ReplaceAllString(base, "") // Removes all illegal characters matched by the regex from the extracted base name.
	// Replace spaces with underscores for file name safety
	clean = strings.ReplaceAll(clean, " ", "_") // Replaces all spaces in the cleaned string with underscores for better cross-platform compatibility.
	// Return the cleaned file name
	return strings.ToLower(clean) // Converts the entire cleaned filename to lowercase and returns it.
}

func main() { // Defines the main function, the entry point of the program
	// The file name where the scraped HTML content will be saved
	outputHTMLFile := "ecolab-com.html" // Defines a string constant for the HTML output filename
	// The urls only file name
	outputURLsFile := "ecolab-com-links.txt" // Defines a string constant for the discovered URLs output filename
	if !fileExists(outputHTMLFile) {         // Calls a (presumed) helper function to check if the HTML file does NOT exist
		// Start the scraping process
		scrapeContentAndSaveToFile(outputHTMLFile)      // Calls a (presumed) function to perform the web scraping and save the result
		log.Println("Scraping completed successfully.") // Logs a message indicating successful scraping
	}
	// Read the scraped HTML content from the file
	htmlContent := readAFileAsString(outputHTMLFile) // Reads the content of the HTML file into the 'htmlContent' string
	// Extract download links from the HTML content
	downloadLinks := extractDownloadLinks(htmlContent) // Calls the function to use regex to extract all PDF links from the HTML content
	// The folder where the downloaded files will be saved
	downloadFolder := "PDFs" // Defines a string constant for the target folder name for PDF downloads
	// Remove duplicates from the extracted download links
	downloadLinks = removeDuplicatesFromSlice(downloadLinks) // Calls the function to remove any duplicate URLs from the extracted links
	// Read the output URLs file to check if it exists
	readOutPutURLsFile := readAFileAsString(outputURLsFile) // Reads the existing content of the URLs tracking file (if it exists)
	for _, link := range downloadLinks {                    // Starts a loop to iterate over each unique PDF link
		fileName := getFileNamesFromURLs(link)          // Calls the function to extract and sanitize a filename from the current URL
		fullPath := path.Join(downloadFolder, fileName) // Constructs the full local path for the PDF: "PDFs/filename"
		if fileExists(fullPath) {                       // Calls a (presumed) helper function to check if the PDF file already exists locally
			log.Printf("File %s already exists, skipping download.", fullPath) // Logs that the file already exists
			continue                                                           // Skips the rest of the loop iteration and moves to the next link
		}
		time.Sleep(1 * time.Second)              // Pauses execution for 1 second to be polite to the server
		err := downloadPDF(link, downloadFolder) // Calls the function to download the PDF to the specified folder
		if err != nil {                          // Checks if the downloadPDF function returned an error
			log.Println("Error downloading PDF:", err) // Logs the error if the PDF download failed
		}
		if !strings.Contains(readOutPutURLsFile, link) { // Checks if the current link string is NOT already present in the content read from the URLs file
			log.Println("Appending link to file:", link)        // Logs a message that the link is being appended
			appendByteToFile(outputURLsFile, []byte(link+"\n")) // Appends the link followed by a newline character to the URLs tracking file
		}
	} // End of the loop
} // End of the main function
