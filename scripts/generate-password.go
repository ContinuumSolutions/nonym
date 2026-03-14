package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := "admin123"

	// Generate hash with bcrypt (cost 12)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		fmt.Printf("Error generating hash: %v\n", err)
		return
	}

	fmt.Printf("Password: %s\n", password)
	fmt.Printf("Hash: %s\n", string(hash))

	// Verify the hash works
	err = bcrypt.CompareHashAndPassword(hash, []byte(password))
	if err != nil {
		fmt.Printf("Hash verification failed: %v\n", err)
	} else {
		fmt.Printf("Hash verification: SUCCESS\n")
	}
}