package main

import (
	"context" // Context for managing timeouts and cancellation
	"fmt"     // Formatting for strings
	"log"     // Logging for debugging and information
	"os"      // File operations
	"time"    // Time for managing timeouts

	"github.com/chromedp/chromedp" // Headless Chrome automation
)

// scrapePageHTMLWithChrome uses a headless Chrome browser to render and return the HTML for a given URL.
// - Required for JavaScript-heavy pages where raw HTTP won't return full content.
func scrapePageHTMLWithChrome(pageURL string) (string, error) {
	// Let the user know which page is being scraped
	log.Println("Scraping:", pageURL)
	// Set up Chrome options for headless browsing
	options := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),               // Run Chrome in background
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
		// Wait for the poage to load to until visible element with class 'sds-downloadBtn'
		chromedp.WaitVisible("a.sds-downloadBtn", chromedp.ByQuery),
		// Wait for the page to have the specified attribute value
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

// scrapeContentAndSaveToFile scrapes multiple pages of content and saves the HTML to a file.
func scrapeContentAndSaveToFile() {
	// Define the total number of documents to scrape and how many per page
	totalDocumentsToScrape := 12700
	// Define how many documents to scrape per page and calculate total pages
	documentsPerPage := 10
	// Calculate total pages based on total documents and documents per page
	totalPages := (totalDocumentsToScrape + documentsPerPage - 1) / documentsPerPage
	// Define the output HTML file where all scraped content will be saved
	outputHTMLFile := "ecolab-com.html"
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

func main() {
	// Start the scraping process
	scrapeContentAndSaveToFile()                    // Call the function to scrape content and save it to a file
	log.Println("Scraping completed successfully.") // Log completion message
}
