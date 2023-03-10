package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

type Record struct {
	TimeTaken    string
	NatureOfCall string
	Disposition  string
	Location     string
	City         string
}

func main() {
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

	// Find the element with ID "isPasted"
	table := findNodeById(doc, "isPasted")
	if table == nil {
		fmt.Println("table not found")
		return
	}

	//fmt.Printf("%s\n", htmlRender(table))

	records := []Record{}

	// parse each tr from the table
	for c := table.FirstChild.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "tr" {
			record := Record{}

			current := c.FirstChild
			record.TimeTaken = strings.TrimSpace(current.FirstChild.Data)
			current = current.NextSibling
			record.NatureOfCall = current.FirstChild.Data
			current = current.NextSibling
			record.Disposition = current.FirstChild.Data
			current = current.NextSibling
			record.Location = current.FirstChild.Data
			current = current.NextSibling
			if current.FirstChild.Data != "br" {
				record.Location += " " + current.FirstChild.Data
			}
			current = current.NextSibling
			record.City = current.FirstChild.Data

			// first record is the header, skip it
			if record.TimeTaken != "TIME TAKEN" {
				records = append(records, record)
			}
		}
	}

	fmt.Printf("%v\n", records)

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
