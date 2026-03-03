package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/joho/godotenv"
)

// SlotRequest is the payload for scanning available slots
type SlotRequest struct {
	CourseType      string      `json:"courseType"`
	InsInstructorId string      `json:"insInstructorId"`
	StageSubDesc    string      `json:"stageSubDesc"`
	SubStageSubNo   interface{} `json:"subStageSubNo"`
	SubVehicleType  interface{} `json:"subVehicleType"`
}

// SlotResponse represents a single slot from the API response
type SlotResponse struct {
	Date       string `json:"date"`       // adjust field names based on actual response
	StartTime  string `json:"startTime"`  // adjust field names based on actual response
	EndTime    string `json:"endTime"`    // adjust field names based on actual response
	Instructor string `json:"instructor"` // optional
}

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file:", err)
	}

	botToken := os.Getenv("TELEGRAM_TOKEN")
	chatIDStr := os.Getenv("CHAT_ID")
	apiToken := os.Getenv("BBDC_API_TOKEN")

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		log.Fatal("Invalid CHAT_ID:", err)
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal("Failed to start Telegram bot:", err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Repeatedly scan every X minutes
	interval := 5 * time.Minute
	for {
		slots, err := fetchAvailableSlots(apiToken)
		if err != nil {
			log.Println("Error fetching slots:", err)
		} else if len(slots) > 0 {
			for _, slot := range slots {
				msg := fmt.Sprintf("Available slot on %s from %s to %s", slot.Date, slot.StartTime, slot.EndTime)
				if slot.Instructor != "" {
					msg += " with instructor " + slot.Instructor
				}
				sendTelegramAlert(bot, chatID, msg)
			}
		} else {
			log.Println("No available slots found")
		}

		log.Printf("Sleeping for %v before next check...\n", interval)
		time.Sleep(interval)
	}
}

// fetchAvailableSlots queries BBDC API for available 3A practical slots
func fetchAvailableSlots(apiToken string) ([]SlotResponse, error) {
	url := "https://booking.bbdc.sg/bbdc-back-service/api/booking/c3practical/listC3PracticalSlotReleased"

	payload := SlotRequest{
		CourseType:      "3A",
		InsInstructorId: "",
		StageSubDesc:    "Practical Lesson",
		SubStageSubNo:   nil,
		SubVehicleType:  nil,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("bad response: %s", string(respBody))
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var slots []SlotResponse
	err = json.Unmarshal(respBody, &slots)
	if err != nil {
		return nil, err
	}

	return slots, nil
}

// sendTelegramAlert sends a message to your Telegram chat
func sendTelegramAlert(bot *tgbotapi.BotAPI, chatID int64, msg string) {
	message := tgbotapi.NewMessage(chatID, msg)
	_, err := bot.Send(message)
	if err != nil {
		log.Println("Error sending Telegram message:", err)
	} else {
		log.Println("Sent alert:", msg)
	}
}
