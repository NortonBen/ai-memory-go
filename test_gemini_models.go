package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	key := os.Getenv("GEMINI_API_KEY")
	resp, err := http.Get("https://generativelanguage.googleapis.com/v1beta/models?key=" + key)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}
