package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/joho/godotenv"
)

// SlotRequest represents the payload to fetch available slots
type SlotRequest struct {
	CourseType      string      `json:"courseType"`
	InsInstructorId string      `json:"insInstructorId"`
	StageSubDesc    string      `json:"stageSubDesc"`
	SubStageSubNo   interface{} `json:"subStageSubNo"`
	SubVehicleType  interface{} `json:"subVehicleType"`
}

// Slot represents a single slot returned by the API
type Slot struct {
	BookingDate string `json:"bookingDate"`
	StartTime   string `json:"startTime"`
	EndTime     string `json:"endTime"`
	Instructor  string `json:"instructorName"`
	Vehicle     string `json:"vehicleType"`
}

// ApiResponse represents the JSON response structure
type ApiResponse struct {
	Data []Slot `json:"data"`
}

func main() {
	// Load .env variables
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, relying on environment variables")
	}

	botToken := os.Getenv("TELEGRAM_TOKEN")
	chatID, _ := strconv.ParseInt(os.Getenv("CHAT_ID"), 10, 64)
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal("Failed to start telegram bot:", err)
	}
	log.Printf("Telegram bot authorized on account %s", bot.Self.UserName)

	// Run web server for Heroku ping
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Bot is running"))
		})
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		log.Fatal(http.ListenAndServe(":"+port, nil))
	}()

	// Start main loop
	for {
		checkSlots(bot, chatID)
		// Random interval between 2–5 minutes
		r := rand.Intn(180) + 120
		log.Printf("Next scan in %d seconds", r)
		time.Sleep(time.Duration(r) * time.Second)
	}
}

func checkSlots(bot *tgbotapi.BotAPI, chatID int64) {
	apiURL := "https://booking.bbdc.sg/bbdc-back-service/api/booking/c3practical/listC3PracticalSlotReleased"

	token := os.Getenv("BBDC_TOKEN")
	if token == "" {
		log.Fatal("BBDC_TOKEN not set")
	}

	// Prepare request payload
	payload := SlotRequest{
		CourseType:      "3A",
		InsInstructorId: "",
		StageSubDesc:    "Practical Lesson",
		SubStageSubNo:   nil,
		SubVehicleType:  nil,
	}
	jsonPayload, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		log.Println("Error creating request:", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var apiResp ApiResponse
	err = json.Unmarshal(body, &apiResp)
	if err != nil {
		log.Println("Error parsing JSON:", err)
		return
	}

	if len(apiResp.Data) == 0 {
		log.Println("No slots available")
		sendTelegram(bot, chatID, "No slots available at this time")
		return
	}

	for _, slot := range apiResp.Data {
		msg := fmt.Sprintf("Available slot on %s from %s to %s\nInstructor: %s\nVehicle: %s",
			slot.BookingDate, slot.StartTime, slot.EndTime, slot.Instructor, slot.Vehicle)
		sendTelegram(bot, chatID, msg)
	}
}

func sendTelegram(bot *tgbotapi.BotAPI, chatID int64, msg string) {
	message := tgbotapi.NewMessage(chatID, msg)
	_, err := bot.Send(message)
	if err != nil {
		log.Println("Failed to send telegram message:", err)
	} else {
		log.Println("Sent Telegram message:", msg)
	}
}
