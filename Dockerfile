# Этап сборки (builder)
FROM golang:1.24 as builder

WORKDIR /app

# Копируем только файлы зависимостей (для кэширования)
COPY go.mod go.sum ./
RUN go mod download


COPY /etc/letsencrypt/live/amber.thatusualguy.ru /etc/letsencrypt/live/amber.thatusualguy.ru 

# Копируем весь код и собираем
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/main ./api/main.go

LABEL maintainer="Alexey Borisoglebsky <endline00@ya.ru>"

# Финальный этап (минимальный образ)
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/main .

EXPOSE 8080
CMD ["./main"]