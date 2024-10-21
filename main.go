package main

import (
	"encoding/json"
	"mariposa/db"
	"mariposa/models"
	"os"

	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

func main() {
	webhookURL := os.Getenv("WEBHOOK_URL")
	if webhookURL == "" {
		fmt.Println("WEBHOOK_URL is not set")
		return
	}

	dbconn, err := db.Init()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer dbconn.Close()

	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://www.mariposacounty.org/690/Daily-Log", nil)
	if err != nil {
		fmt.Println("Request creation error:", err)
		os.Exit(1)
	}

	// Set User-Agent
    //req.Header.Set("User-Agent", "curl/7.88.1")
	req.Header.Set("User-Agent", "fafrd/mariposabot")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Request error:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// Exit early if 404 is returned
	if resp.StatusCode == http.StatusNotFound {
		fmt.Println("404 Not Found")
		os.Exit(1)
	}


	// Read the response body into a byte slice
	body, err := io.ReadAll(resp.Body)
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
		fmt.Printf("Processing %s\n", date)
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
			callForService := strings.TrimSpace(current.FirstChild.Data)
			// Skip rows without a valid "CALL FOR SERVICE"
			if callForService == "" {
				continue
			}

			// Split the "CALL FOR SERVICE" by "-" and take the second part
			parts := strings.Split(callForService, "-")
			if len(parts) < 2 {
				fmt.Println("Skipping invalid CALL FOR SERVICE:", callForService)
				continue
			}
			record.NatureOfCall = strings.TrimSpace(parts[1])

			// TimeTaken will be just the date with 00:00 appended
			record.TimeTaken = date + " 00:00"

			// Skip the disposition, only take the "City" which is now the second column
			current = current.NextSibling
			if current.FirstChild.Data == "br" {
				record.City = ""
			} else {
				record.City = current.FirstChild.Data
			}

			// Add the record to the list
			records = append(records, record)
		}
	}

	fmt.Printf("Found %d log entries\n", len(records))

	// for each record, call db.InsertRecord and send to webhook
	for _, record := range records {
		err = db.InsertRecord(dbconn, record)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	// emit payload to webhook
	if len(records) > 0 {
		var builder strings.Builder

		builder.WriteString("```")
		builder.WriteString(fmt.Sprintf("Police log for %s\n", strings.SplitN(records[0].TimeTaken, " ", 2)[0]))
		builder.WriteString("\n")
		for _, record := range records {
			builder.WriteString(record.NatureOfCall)
			builder.WriteString("\n")
		}
		builder.WriteString("```")

		// Get the final string
		finalStr := builder.String()

		// Send the final string to the webhook
		err = sendToWebhook(webhookURL, finalStr)
		if err != nil {
			fmt.Println("Error sending to webhook:", err)
			return
		}
	}

	if !exists {
		// insert a record into days_processed table to prevent duplicate processing
		fmt.Printf("Processed %s\n", date)
		err := db.InsertDate(dbconn, date)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
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
	if n == nil {
		return ""
	}
	var buf bytes.Buffer
	w := io.Writer(&buf)
	html.Render(w, n)
	return buf.String()
}

type WebhookData struct {
	Username string `json:"username"`
	Content  string `json:"content"`
}

func sendToWebhook(url string, content string) error {
	webhookData := WebhookData{
		Username: "mariposa",
		Content:  content,
	}

	jsonData, err := json.Marshal(webhookData)
	if err != nil {
		return err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("received non-OK response code: %d, body: %s", resp.StatusCode, body)
	}

	return nil
}
