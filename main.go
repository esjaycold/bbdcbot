package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const endpoint = "https://booking.bbdc.sg/bbdc-back-service/api/booking/c3practical/listC3PracticalTrainings"

type SlotRequest struct {
	CourseType      string      `json:"courseType"`
	InsInstructorId string      `json:"insInstructorId"`
	StageSubDesc    string      `json:"stageSubDesc"`
	SubStageSubNo   interface{} `json:"subStageSubNo"`
	SubVehicleType  interface{} `json:"subVehicleType"`
}

func main() {
	telegramToken := os.Getenv("TELEGRAM_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	bbdcToken := os.Getenv("BBDC_TOKEN")

	if telegramToken == "" || chatID == "" || bbdcToken == "" {
		log.Fatal("Missing TELEGRAM_TOKEN, TELEGRAM_CHAT_ID, or BBDC_TOKEN")
	}

	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Telegram bot authorized: %s", bot.Self.UserName)

	// Heroku requires web server
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Bot is running"))
		})
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		http.ListenAndServe(":"+port, nil)
	}()

	for {
		checkSlots(bot, chatID, bbdcToken)
		log.Println("Next check in 240 seconds")
		time.Sleep(240 * time.Second)
	}
}

func checkSlots(bot *tgbotapi.BotAPI, chatID string, token string) {

	payload := SlotRequest{
		CourseType:      "3A",
		InsInstructorId: "",
		StageSubDesc:    "Practical Lesson",
		SubStageSubNo:   nil,
		SubVehicleType:  nil,
	}

	jsonData, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Println("Request creation error:", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Origin", "https://booking.bbdc.sg")
	req.Header.Set("Referer", "https://booking.bbdc.sg/")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Request error:", err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyString := string(bodyBytes)

	if resp.StatusCode != 200 {
		log.Println("Non-200 response:", resp.Status)
		log.Println("Response body:", bodyString)
		return
	}

	// Try parsing JSON
	var parsed interface{}
	err = json.Unmarshal(bodyBytes, &parsed)
	if err != nil {
		log.Println("JSON parsing failed. Raw response below:")
		log.Println(bodyString)
		return
	}

	// Simple detection: if response contains slot data
	if len(bodyBytes) > 200 {
		msg := tgbotapi.NewMessage(parseChatID(chatID),
			"🚨 BBDC SLOT POSSIBLY AVAILABLE!\n\nCheck now: https://booking.bbdc.sg/")
		bot.Send(msg)
		log.Println("Telegram alert sent.")
	} else {
		log.Println("No slots found.")
	}
}

func parseChatID(chatID string) int64 {
	var id int64
	fmt.Sscan(chatID, &id)
	return id
}
