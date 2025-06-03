package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"tg_bot/auth"
	db "tg_bot/internal/database"
	"tg_bot/internal/handlers"
	"tg_bot/internal/utils"

	"tg_bot/models"

	"gorm.io/gorm"
)

type (
	CallbackQuery = models.CallbackQuery
	CallBackData  = models.CallBackData
	Message       = models.Message
	Update        = models.Update
)

var (
	code_request   = utils.Code_request
	decode_request = utils.Decode_request
)

var DB *gorm.DB

var adminStates map[int64]int

func init() {
	var err error
	DB, err = db.Connect()
	if err != nil {
		log.Panic("Ошибка подключения к базе:", err)
	}
	DB.AutoMigrate(&models.CatalogItem{}, &models.Order{}, &models.ProductInStock{})
	DB.Migrator().CreateIndex(&models.CatalogItem{}, "idx_category_manufacturer_model") // Создает составной индекс из полей, помеченных тэгом
	db.SeedTestData(DB)
	adminStates = auth.GetAuth().GetAdminStates()
	//DB.AutoMigrate(&models.User{}, &models.CatalogItem{}, &models.Transaction{})

	botToken = os.Getenv("BOT_TOKEN")
	webhook_url = os.Getenv("BOT_URL")
}

var botToken, webhook_url string

func main() {

	if len(os.Args) > 1 && os.Args[1] == "setwebhook" {
		utils.SetWebhook(webhook_url)
		fmt.Println(webhook_url)
		return
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { handlers.StartHandler(DB, w, r) })
	fmt.Println("Server is running on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
