package main

import (
	"log"

	"hero_budget_backend/utils"
)

func main() {
	log.Println("Iniciando reparación de emojis en categorías...")

	if err := utils.FixEmojis(); err != nil {
		log.Fatalf("Error al reparar emojis: %v", err)
	}

	log.Println("Reparación de emojis completada con éxito")
}
