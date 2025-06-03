package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"tg_bot/models"
)

var BotToken = os.Getenv("BOT_TOKEN")

func Code_request(data models.CallBackData) string { // кодирует запрос в json строку
	return strings.Join([]string{data.Command, data.Category, data.Manufacturer, data.Model, data.ModelID}, "|")
}
func Decode_request(encoded string) models.CallBackData {
	parts := strings.Split(encoded, "|")

	data := models.CallBackData{}

	if len(parts) > 0 {
		data.Command = parts[0]
	}
	if len(parts) > 1 {
		data.Category = parts[1]
	}
	if len(parts) > 2 {
		data.Manufacturer = parts[2]
	}
	if len(parts) > 3 {
		data.Model = parts[3]
	}
	if len(parts) > 4 {
		data.ModelID = parts[4]
	}

	return data
}

func SetWebhook(webhookURL string) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook", BotToken)

	data := map[string]string{"url": webhookURL}
	json_data, _ := json.Marshal(data)

	resp, err := http.Post(apiURL, "application/json", bytes.NewReader(json_data))
	if err != nil {
		log.Fatal("Ошибка при установке webhook:", err)
	}
	defer resp.Body.Close()

	log.Println("Webhook установлен:", resp.Status)
}

func DeleteMessage(chatID int64, msgID int64) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/deleteMessage", BotToken)

	payload := map[string]interface{}{
		"chat_id":    strconv.Itoa(int(chatID)),
		"message_id": strconv.Itoa(int(msgID)),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Println("Error marshaling JSON:", err)
		return
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Println("Error deleting message:", err)
		return
	}
	defer resp.Body.Close()
}

func SendMessage(chatID int64, text string) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", BotToken)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Println("Error marshaling JSON:", err)
		return
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Println("Error sending message:", err)
		return
	}
	defer resp.Body.Close()
}
