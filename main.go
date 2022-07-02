package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/namsral/flag"
	log "github.com/sirupsen/logrus"
)

var (
	logger        = log.New()
	buildVersion  = "development"
	address       *string
	db            *string
	frequency     *int
	redisAddress  *string
	redisPassword *string
	redisDB       *int
	cache         *bool
	managed       *bool
	version       *bool
	rdb           *redis.Client
	ctx           context.Context
	tickerFolder  *string
)

func init() {
	logLevel := flag.Int("logLevel", 0, "defines the log level: 0=INFO 1=DEBUG")
	address = flag.String("address", "0.0.0.0:8080", "address:port to bind http server to")
	db = flag.String("db", "", "file to store tickers in")
	frequency = flag.Int("frequency", 0, "set frequency for all tickers")
	redisAddress = flag.String("redisAddress", "localhost:6379", "address:port for redis server")
	redisPassword = flag.String("redisPassword", "", "redis password")
	redisDB = flag.Int("redisDB", 0, "redis db to use")
	cache = flag.Bool("cache", false, "enable cache for coingecko")
	managed = flag.Bool("managed", false, "forcefully keep db and discord updated with bot values")
	version = flag.Bool("version", false, "print version")
	tickerFolder = flag.String("tickerFolder", "./discord-bot-configs", "ticker configs in here will be used to auto create bot tickers")
	flag.Parse()

	// init logger
	logger.Out = os.Stdout
	switch *logLevel {
	case 0:
		logger.SetLevel(log.InfoLevel)
	default:
		logger.SetLevel(log.DebugLevel)
	}
}

func main() {
	var wg sync.WaitGroup

	if *version {
		logger.Infof("discord-stock-ticker@%s\n", buildVersion)
		return
	}

	logger.Infof("Running discord-stock-ticker version %s...", buildVersion)

	// Redis is used a an optional cache for coingecko data
	if *cache {
		rdb = redis.NewClient(&redis.Options{
			Addr:     *redisAddress,
			Password: *redisPassword,
			DB:       *redisDB,
		})
		ctx = context.Background()
	}

	// Create the bot manager
	wg.Add(1)
	NewManager(*address, *db, tickerCount, rdb, ctx)

	autoCreateTickers()

	// wait forever
	wg.Wait()
}

func autoCreateTickers() {
	configDirectory := *tickerFolder
	files, err := ioutil.ReadDir(configDirectory)
	if err != nil {
		log.Fatal(err)
	}

	r, _ := regexp.Compile(`\.sample\.`)
	for _, file := range files {
		if !file.IsDir() && r.FindString(file.Name()) == "" {
			fileFullPath := fmt.Sprintf("%s/%s", configDirectory, file.Name())
			jsonFile, err := os.Open(fileFullPath)
			if err != nil {
				fmt.Println("Error reading json file: ", file.Name())
			}
			createTicker(jsonFile)
		}
	}
}

func createTicker(jsonFile *os.File) {
	reqURL := fmt.Sprintf("http://%s/ticker", *address)
	byteValue, _ := ioutil.ReadAll(jsonFile)
	req, err := http.NewRequest("POST", reqURL, bytes.NewBuffer(byteValue))
	if err != nil {
		fmt.Println("Error making request: ", err)
	}
	req.Header.Add("Content-Type", "application/json")
	client := &http.Client{}
	_, err = client.Do(req)
	if err != nil {
		fmt.Println("Error: ", err)
	}
}
