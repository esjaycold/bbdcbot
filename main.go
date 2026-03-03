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
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// SlotRequest is the payload to query available slots
type SlotRequest struct {
	CourseType      string      `json:"courseType"`
	InsInstructorId string      `json:"insInstructorId"`
	StageSubDesc    string      `json:"stageSubDesc"`
	SubStageSubNo   interface{} `json:"subStageSubNo"`
	SubVehicleType  interface{} `json:"subVehicleType"`
}

func main() {
	// Telegram setup
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_TOKEN"))
	errCheck(err, "Failed to start Telegram bot")
	log.Printf("Telegram bot authorized: %s", bot.Self.UserName)

	chatID, err := strconv.ParseInt(os.Getenv("CHAT_ID"), 10, 64)
	errCheck(err, "Failed to fetch chat ID")

	// Heroku ping endpoint (to keep awake)
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Bot is running"))
		})
		log.Fatal(http.ListenAndServe(":"+os.Getenv("PORT"), nil))
	}()

	bbdcToken := os.Getenv("BBDC_TOKEN")
	if bbdcToken == "" {
		log.Fatal("BBDC_TOKEN not set")
	}

	for {
		checkSlots(bbdcToken, bot, chatID)
		// Random delay between 2–5 minutes
		r := rand.Intn(180) + 120
		log.Printf("Next check in %d seconds", r)
		time.Sleep(time.Duration(r) * time.Second)
	}
}

func checkSlots(bbdcToken string, bot *tgbotapi.BotAPI, chatID int64) {
	url := "https://booking.bbdc.sg/bbdc-back-service/api/booking/c3practical/listC3PracticalSlotReleased"

	payload := SlotRequest{
		CourseType:      "3A",
		InsInstructorId: "",
		StageSubDesc:    "Practical Lesson",
		SubStageSubNo:   nil,
		SubVehicleType:  nil,
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	errCheck(err, "Error creating request")

	req.Header.Set("Authorization", "Bearer "+bbdcToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	errCheck(err, "Error fetching slots")
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error response from BBDC API: %s", resp.Status)
		return
	}

	var slots []map[string]interface{}
	if err := json.Unmarshal(respBody, &slots); err != nil {
		log.Println("Error parsing response JSON:", err)
		return
	}

	if len(slots) == 0 {
		log.Println("No slots available")
		return
	}

	// Filter by environment variables
	wantedMonths := strings.Split(os.Getenv("WANTED_MONTHS"), ",")
	wantedSessions := strings.Split(os.Getenv("WANTED_SESSIONS"), ",")
	wantedDays := strings.Split(os.Getenv("WANTED_DAYS"), ",")

	for _, slot := range slots {
		dateStr := fmt.Sprintf("%v", slot["date"]) // adjust field name if API returns something else
		session := fmt.Sprintf("%v", slot["session"])
		dayOfWeek := fmt.Sprintf("%v", slot["dayOfWeek"])

		if contains(wantedMonths, dateStr) && contains(wantedSessions, session) && contains(wantedDays, dayOfWeek) {
			alertMsg := fmt.Sprintf("Available slot on %s (Session %s, Day %s)", dateStr, session, dayOfWeek)
			alert(alertMsg, bot, chatID)
		}
	}
}

func alert(msg string, bot *tgbotapi.BotAPI, chatID int64) {
	telegramMsg := tgbotapi.NewMessage(chatID, msg)
	if _, err := bot.Send(telegramMsg); err != nil {
		log.Println("Error sending Telegram message:", err)
	} else {
		log.Println("Sent Telegram message:", msg)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.Contains(item, s) {
			return true
		}
	}
	return false
}

func errCheck(err error, msg string) {
	if err != nil {
		log.Fatal(msg+": ", err)
	}
}
