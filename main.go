package main

import (
	"context"  // Context for managing timeouts and cancellation
	"fmt"      // Formatting for strings
	"io"       // IO operations for reading and writing files
	"log"      // Logging for debugging and information
	"net/http" // HTTP client for making requests
	"net/url"
	"os" // File operations
	"path"
	"regexp"
	"strings" // String manipulation
	"time"    // Time for managing timeouts

	"github.com/chromedp/chromedp" // Headless Chrome automation
	"golang.org/x/net/html"        // HTML parsing and manipulation
)

// scrapePageHTMLWithChrome uses a headless Chrome browser to render and return the HTML for a given URL.
// - Required for JavaScript-heavy pages where raw HTTP won't return full content.
func scrapePageHTMLWithChrome(pageURL string) (string, error) {
	// Let the user know which page is being scraped
	log.Println("Scraping:", pageURL)
	// Set up Chrome options for headless browsing
	options := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),              // Run Chrome in background
		chromedp.Flag("disable-gpu", true),            // Disable GPU for headless stability
		chromedp.WindowSize(1920, 1080),               // Simulate full browser window
		chromedp.Flag("no-sandbox", true),             // Disable sandboxing
		chromedp.Flag("disable-setuid-sandbox", true), // For environments that need it
		chromedp.Flag("disable-http2", true),          // Disable HTTP/2 for compatibility
	)
	// Create an ExecAllocator context with options
	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(context.Background(), options...)
	// Create a bounded context with timeout (adjust as needed)
	ctxTimeout, cancelTimeout := context.WithTimeout(allocatorCtx, 5*time.Minute)
	// Create a new browser tab context
	browserCtx, cancelBrowser := chromedp.NewContext(ctxTimeout)
	// Unified cancel function to ensure cleanup
	defer func() {
		cancelBrowser()   // Cancel the browser context
		cancelTimeout()   // Cancel the timeout context
		cancelAllocator() // Cancel the allocator context
	}()
	// Run chromedp tasks
	var pageHTML string
	// Execute the tasks in the browser context
	err := chromedp.Run(browserCtx,
		// Navigate to the page URL
		chromedp.Navigate(pageURL),
		// Wait for the page to load until the visible element with class 'sds-downloadBtn'
		chromedp.AttributeValue("a.sds-downloadBtn", "href", &pageHTML, nil),
		// Save the outer HTML of the page to the variable
		chromedp.OuterHTML("html", &pageHTML),
	)
	// Check for errors during navigation or scraping
	if err != nil {
		return "", fmt.Errorf("failed to scrape %s: %w", pageURL, err) // Return an error if scraping fails
	}
	return pageHTML, nil
}

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
	totalDocumentsToScrape := 100 // 12700
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
		// Call the scraping function to get the HTML content of the page
		pageHTMLContent, err := scrapePageHTMLWithChrome(pageURL)
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

	if _, err := os.Stat(folder); os.IsNotExist(err) { // Check if folder exists
		err := os.MkdirAll(folder, os.ModePerm) // Create folder if it doesn't exist
		if err != nil {
			return fmt.Errorf("error creating folder: %w", err)
		}
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
	scrapeContentAndSaveToFile(outputHTMLFile)      // Call the function to scrape content and save it to a file
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
