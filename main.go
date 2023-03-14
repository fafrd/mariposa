package main

import (
	"mariposa/db"
	"mariposa/models"

	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

func main() {
	dbconn, err := db.Init()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer dbconn.Close()
	
	// Define the URL of the webpage to scrape
	url := "https://www.mariposacounty.org/690/Daily-Log"

	// Send an HTTP GET request to the URL
	response, err := http.Get(url)
	if err != nil {
		fmt.Println("Error fetching URL:", err)
		return
	}
	defer response.Body.Close()

	// Read the response body into a byte slice
	body, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	// Parse the HTML document
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		fmt.Println("Error parsing HTML:", err)
		return
	}

	//fmt.Printf("%s\n", htmlRender(doc))

	// find the date
	versionHeadLine := findNodeById(doc, "versionHeadLine")
	re := regexp.MustCompile(`\d{2}-\d{2}-\d{4}`)
    date := re.FindString(htmlRender(versionHeadLine))

	// is the date already in the db?
	exists, err := db.DateExists(dbconn, date)
	if err != nil {
		fmt.Println(err)
		return
	}
	if exists {
		fmt.Printf("Already processed %s\n", date)
		return
	} else {
		// insert a record into days_processed table to prevent duplicate processing
		fmt.Printf("Processing %s\n", date)
		err := db.InsertDate(dbconn, date)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	// Find the element with ID "isPasted"
	table := findNodeById(doc, "isPasted")
	if table == nil {
		fmt.Println("table not found")
		return
	}

	//fmt.Printf("%s\n", htmlRender(table))

	records := []models.Record{}

	// parse each tr from the table
	for c := table.FirstChild.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "tr" {
			record := models.Record{}

			current := c.FirstChild
			record.TimeTaken = strings.TrimSpace(current.FirstChild.Data)
			// first record is the header, skip it
			if strings.EqualFold(record.TimeTaken, "TIME TAKEN") {
				continue
			}
			record.TimeTaken = date + " " + record.TimeTaken

			current = current.NextSibling
			if current.FirstChild.Data == "br" {
				record.NatureOfCall = ""
			} else {
				record.NatureOfCall = current.FirstChild.Data
			}

			current = current.NextSibling
			if current.FirstChild.Data == "br" {
				record.Disposition = ""
			} else {
				record.Disposition = current.FirstChild.Data
			}

			// next col is location 1, following col is location 2, after that is city
			// concat location 1 + location 2 if nonempty
			current = current.NextSibling
			if current.FirstChild.Data == "br" {
				record.Location = ""
				current = current.NextSibling // advance ptr
			} else {
				record.Location = current.FirstChild.Data
				current = current.NextSibling // advance ptr
				if current.FirstChild.Data != "br" {
					record.Location += " " + current.FirstChild.Data
				}
			}

			current = current.NextSibling
			if current.FirstChild.Data == "br" {
				record.City = ""
			} else {
				record.City = current.FirstChild.Data
			}

			records = append(records, record)
		}
	}

	// for each record, call db.InsertRecord
	for _, record := range records {
		err = db.InsertRecord(dbconn, record)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	fmt.Printf("Processed %d records\n", len(records))

}

// Helper function to find a node with a given ID attribute
func findNodeById(n *html.Node, id string) *html.Node {
	if n.Type == html.ElementNode {
		for _, attr := range n.Attr {
			if attr.Key == "id" && attr.Val == id {
				return n
			}
		}
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if result := findNodeById(c, id); result != nil {
			return result
		}
	}

	return nil
}

func htmlRender(n *html.Node) string {
	var buf bytes.Buffer
	w := io.Writer(&buf)
	html.Render(w, n)
	return buf.String()
}
