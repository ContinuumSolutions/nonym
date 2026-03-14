#!/bin/bash
set -e

echo "🔍 Debugging Authentication Issues..."

# Check if the database connection is working from the gateway
echo "📡 Testing database connection from gateway container..."
docker compose exec gateway go run -c '
package main

import (
    "fmt"
    "os"
    "database/sql"
    _ "github.com/lib/pq"
)

func main() {
    dbHost := os.Getenv("DB_HOST")
    dbPort := os.Getenv("DB_PORT")
    dbName := os.Getenv("DB_NAME")
    dbUser := os.Getenv("DB_USER")
    dbPassword := os.Getenv("DB_PASSWORD")

    dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
        dbHost, dbPort, dbUser, dbPassword, dbName)

    db, err := sql.Open("postgres", dsn)
    if err != nil {
        fmt.Printf("Error opening database: %v\n", err)
        return
    }
    defer db.Close()

    if err := db.Ping(); err != nil {
        fmt.Printf("Error pinging database: %v\n", err)
        return
    }

    fmt.Println("Database connection successful!")

    // Check if user exists
    var count int
    err = db.QueryRow("SELECT COUNT(*) FROM users WHERE email = $1", "admin@localhost").Scan(&count)
    if err != nil {
        fmt.Printf("Error querying users: %v\n", err)
        return
    }

    fmt.Printf("Found %d users with email admin@localhost\n", count)
}
'