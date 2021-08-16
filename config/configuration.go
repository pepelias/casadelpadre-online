package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"sync"
)

// Configuration .
type Configuration struct {
	SSL struct {
		Cert string `json:"cert"`
		Key  string `json:"key"`
	} `json:"ssl"`
	Server struct {
		Port       string   `json:"port"`
		SecurePort string   `json:"secure_port"`
		Cors       []string `json:"cors"`
	} `json:"server"`
	Qualities struct {
		High string `json:"high"`
		Mid  string `json:"mid"`
		Low  string `json:"low"`
	} `json:"qualities"`
}

var (
	config Configuration
	once   sync.Once
)

// Get obtiene la configuración
func Get() *Configuration {
	once.Do(load)
	return &config
}
func load() {
	log.Println("Leyendo el archivo de configuración...")
	b, err := ioutil.ReadFile("configuration.json")
	if err != nil {
		log.Fatalf("Error al leer el archivo de configuración: %s", err.Error())
	}

	err = json.Unmarshal(b, &config)
	if err != nil {
		log.Fatalf("Error al parsear el archivo de configuración: %s", err.Error())
	}

	log.Println("Archivo de configuración cargado.")
}
