package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Define your models (StackOverflowPost, GitHubIssue) here as shown earlier

//	type StackOverflowPost struct {
//		QuestionID int    `json:"question_id"`
//		Title      string `json:"title"`
//		Body       string `json:"body"`
//		Answers    []struct {
//			AnswerID int    `json:"answer_id"`
//			Body     string `json:"body"`
//		} `json:"answers"`
//	}
type StackOverflowPost struct {
	QuestionID int    `json:"question_id"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	Answers    string `json:"answers"` // Store JSON as a string
}

type GitHubIssue struct {
	ID     int    `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	// include other fields as per the JSON response
}

type PostData struct {
	Title   string
	Content string
	Tags    []string
}

var (
	githubAPICalls = promauto.NewCounter(prometheus.CounterOpts{
		Name: "myapp_github_api_calls_total",
		Help: "Total number of API calls to GitHub",
	})
	stackoverflowAPICalls = promauto.NewCounter(prometheus.CounterOpts{
		Name: "myapp_stackoverflow_api_calls_total",
		Help: "Total number of API calls to StackOverflow",
	})
	dataCollected = promauto.NewCounter(prometheus.CounterOpts{
		Name: "myapp_data_collected_bytes_total",
		Help: "Total amount of data collected in bytes",
	})
)

func main() {
	db := connectDatabase()

	// Fiber App Setup
	app := fiber.New()

	// GET endpoint to trigger data fetching
	app.Get("/fetch-data", func(c *fiber.Ctx) error {
		go fetchDataAndStore(db) // Fetch and store data asynchronously
		return c.SendString("Data fetching initiated")
	})

	// POST endpoint to receive and store data
	app.Post("/store-data", func(c *fiber.Ctx) error {
		var postData PostData // Define PostData struct according to your data format
		if err := c.BodyParser(&postData); err != nil {
			return c.Status(400).SendString(err.Error())
		}
		storeData(db, postData) // Store the data in the database
		return c.SendString("Data stored successfully")
	})

	// Run Fiber App in a Goroutine
	go func() {
		log.Fatal(app.Listen(":3000"))
	}()

	// Prometheus Metrics Server
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":9091", nil))
}

func connectDatabase() *gorm.DB {
	dsn := "host=localhost user=myuser password=mypassword dbname=cs port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Migrate the schema
	db.AutoMigrate(&StackOverflowPost{}, &GitHubIssue{})

	return db
}

func fetchStackOverflowData() []StackOverflowPost {
	var allPosts []StackOverflowPost

	stackoverflowAPICalls.Inc()

	// Define the API endpoint with your parameters
	url := "https://api.stackexchange.com/2.3/search/advanced?order=desc&sort=activity&tagged=prometheus&site=stackoverflow&filter=withbody"

	// Make the GET request
	//key := "W3RncUHVnWXPhdGQQiHYxA(("

	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("Error making request to Stack Overflow API: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response body: %v", err)
	}

	// Add the size of the response body to the data collected counter
	dataCollected.Add(float64(len(body)))

	// Parse the JSON response
	var result struct {
		Items []StackOverflowPost `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Fatalf("Error unmarshaling response JSON: %v", err)
	}

	allPosts = append(allPosts, result.Items...)

	// Handle pagination if necessary
	// print posts as well
	for _, post := range allPosts {
		fmt.Printf("Question ID: %d, Title: %s, Body: %s\n", post.QuestionID, post.Title, post.Body)
	}

	return allPosts
}

func fetchGitHubData() []GitHubIssue {
	// Set your repository details

	githubAPICalls.Inc()
	owner := "prometheus"
	repo := "prometheus"

	// GitHub API URL for issues
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", owner, repo)

	// Create a new HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Error creating request: %v", err)
	}

	// Set your GitHub token for authentication
	// Ensure you have set your token in your environment variables
	req.Header.Set("Authorization", "token "+"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")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Error sending request to GitHub API: %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response body: %v", err)
	}
	dataCollected.Add(float64(len(body)))
	// Unmarshal the JSON data into the slice of GitHubIssue
	var issues []GitHubIssue
	if err := json.Unmarshal(body, &issues); err != nil {
		log.Fatalf("Error unmarshaling response JSON: %v", err)
	}
	for _, issue := range issues {
		fmt.Printf("ID: %d, Number: %d, Title: %s, Body: %s\n", issue.ID, issue.Number, issue.Title, issue.Body)
	}

	return issues
}

func storeStackOverflowPost(db *gorm.DB, post StackOverflowPost) {
	db.Create(&post)
}

func storeGitHubIssue(db *gorm.DB, issue GitHubIssue) {
	var existingIssue GitHubIssue
	result := db.First(&existingIssue, issue.ID)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// Record not found, create a new one
		db.Create(&issue)
	} else {
		// Record exists, update it
		db.Model(&existingIssue).Updates(issue)
	}
}

func fetchDataAndStore(db *gorm.DB) {
	// Fetch data from Stack Overflow API
	stackOverflowPosts := fetchStackOverflowData() // Placeholder for API call
	for _, post := range stackOverflowPosts {
		storeStackOverflowPost(db, post)
	}

	// Fetch data from GitHub API
	gitHubIssues := fetchGitHubData() // Placeholder for API call
	for _, issue := range gitHubIssues {
		storeGitHubIssue(db, issue)
	}
}

func storeData(db *gorm.DB, data PostData) {
	// Assuming PostData is a struct that represents the data you want to store
	// The following is a generic way to store data in a database using GORM
	result := db.Create(&data)
	if result.Error != nil {
		log.Printf("Error storing data: %v", result.Error)
		// Handle the error appropriately
	}
}
