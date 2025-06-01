FROM golang:1.23-alpine

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Сборка приложения
RUN go build -o bot main.go
RUN chmod +x ./bot
# Порт сервера
EXPOSE 8080

# Команда запуска
CMD ./bot setwebhook https://perspective-that-similarly-uncle.trycloudflare.com && \
    ./bot