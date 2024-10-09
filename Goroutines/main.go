package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var dataSources = []string{
	"https://jsonplaceholder.typicode.com/posts/1",
	"https://jsonplaceholder.typicode.com/posts/2",
	"https://jsonplaceholder.typicode.com/posts/3",
}

var mu sync.Mutex

type DataSummary struct {
	ID        uint      `gorm:"primaryKey"`
	Source    string    `gorm:"type:text"`
	Timestamp time.Time `gorm:"autoCreateTime"`
}

func initDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("dashboard.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	err = db.AutoMigrate(&DataSummary{})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func fetchDataFromAPI(url string) (string, error) {
	response, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&data); err != nil {
		return "", err
	}

	return fmt.Sprintf("Data from API: %v", data), nil
}

func collectData(source string, wg *sync.WaitGroup, results chan<- string, db *gorm.DB) {
	defer wg.Done()

	data, err := fetchDataFromAPI(source)
	if err != nil {
		log.Printf("Error fetching data from %s: %v\n", source, err)
		return
	}

	mu.Lock()
	err = db.Create(&DataSummary{Source: data}).Error
	if err != nil {
		log.Printf("Error inserting data: %v\n", err)
	}
	mu.Unlock()

	results <- data
}

func startServer(db *gorm.DB) {
	r := gin.Default()

	r.GET("/data", func(c *gin.Context) {
		var dataSummaries []DataSummary
		if err := db.Find(&dataSummaries).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, dataSummaries)
	})

	r.Run(":8080")
}

func main() {
	db, err := initDB()
	if err != nil {
		log.Fatalf("Error initializing database: %v\n", err)
	}

	var wg sync.WaitGroup
	results := make(chan string, len(dataSources))

	for _, source := range dataSources {
		wg.Add(1)
		go collectData(source, &wg, results, db)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		fmt.Println(result)
	}

	startServer(db)
}
