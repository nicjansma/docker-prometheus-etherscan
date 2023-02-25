package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const LISTEN_ADDRESS = ":9205"
const API_URL_MAINNET = "https://api.etherscan.io/api"
const API_URL_TESTNET = "https://api-{{TESTNET}}.etherscan.io/api"

var testMode string
var accountIds string
var apiKey string
var testNet string

type EtherScanBalanceMulti struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  []struct {
		Account string `json:"account"`
		Balance string `json:"balance"`
	} `json:"result"`
}

type EtherScanEthBlocKNumber struct {
	JsonRpc string `json:"jsonrpc"`
	Id      string `json:"id"`
	Result  string `json:"result"`
}

func integerToString(value int) string {
	return strconv.Itoa(value)
}

func integer64ToString(value int64) string {
	return strconv.FormatInt(value, 10)
}

func baseUnitsToEth(value string, precision int) string {
	if len(value) < precision {
		value = strings.Repeat("0", precision-len(value)) + value
	}
	return value[:len(value)-precision+1] + "." + value[len(value)-precision+1:]
}

func formatValue(key string, meta string, value string) string {
	result := key
	if meta != "" {
		result += "{" + meta + "}"
	}
	result += " "
	result += value
	result += "\n"
	return result
}

func queryData(root string, path string) (string, error) {
	// Build URL
	url := root + path

	// Perform HTTP request
	resp, httpErr := http.Get(url)
	if httpErr != nil {
		return "", httpErr
	}

	// Parse response
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", errors.New("HTTP returned code " + integerToString(resp.StatusCode))
	}
	bodyBytes, ioErr := io.ReadAll(resp.Body)
	bodyString := string(bodyBytes)
	if ioErr != nil {
		return "", ioErr
	}

	return bodyString, nil
}

func getTestData(file string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	body, err := os.ReadFile(dir + "/" + file)
	if err != nil {
		log.Fatal(err)
	}
	return string(body), nil
}

func metrics(w http.ResponseWriter, r *http.Request) {
	log.Print("Serving /metrics")

	up := 1

	var jsonString string
	var jsonStringBlockNumber string
	var jsonStringBlockNumberTestnet string
	var err error
	var ethBlock int64
	var errBlockNumber error
	var errBlockNumberTestnet error

	if testMode == "1" {
		jsonString, err = getTestData("test.json")
		jsonStringBlockNumber, errBlockNumber = getTestData("test-blocknumber.json")
	} else {
		// mainnet queries
		jsonString, err = queryData(API_URL_MAINNET, "?module=account&action=balancemulti&address="+accountIds+"&tag=latest&apikey="+apiKey)
		jsonStringBlockNumber, errBlockNumber = queryData(API_URL_MAINNET, "?module=proxy&action=eth_blockNumber&apikey="+apiKey)

		// optional testnet queries
		if testNet != "" {
			var testNetUrl = strings.Replace(API_URL_TESTNET, "{{TESTNET}}", testNet, -1)
			jsonStringBlockNumberTestnet, errBlockNumberTestnet = queryData(testNetUrl, "?module=proxy&action=eth_blockNumber&apikey="+apiKey)
		}
	}

	if err != nil {
		log.Print(err)
		up = 0
	}

	if errBlockNumber != nil {
		log.Print(errBlockNumber)
		up = 0
	}

	if errBlockNumberTestnet != nil {
		log.Print(errBlockNumberTestnet)
		up = 0
	}

	// Parse JSON
	jsonData := EtherScanBalanceMulti{}
	json.Unmarshal([]byte(jsonString), &jsonData)

	jsonDataBlockNumber := EtherScanEthBlocKNumber{}
	json.Unmarshal([]byte(jsonStringBlockNumber), &jsonDataBlockNumber)

	jsonDataBlockNumberTestnet := EtherScanEthBlocKNumber{}
	json.Unmarshal([]byte(jsonStringBlockNumberTestnet), &jsonDataBlockNumberTestnet)

	// Check response status
	if jsonData.Status != "1" {
		log.Print("Received negative status in JSON response '" + jsonData.Status + "'")
		log.Print(jsonString)
		up = 0
	}

	// Output
	io.WriteString(w, formatValue("etherscan_up", "", integerToString(up)))
	for _, Account := range jsonData.Result {
		io.WriteString(w, formatValue("etherscan_balance", "account=\""+Account.Account+"\"", baseUnitsToEth(Account.Balance, 19)))
	}

	ethBlock, err = strconv.ParseInt(strings.Replace(jsonDataBlockNumber.Result, "0x", "", -1), 16, 64)
	if err != nil {
		log.Print(err)
		up = 0
	} else {
		io.WriteString(w, formatValue("etherscan_block_number{network=\"mainnet\"}", "", integer64ToString(ethBlock)))
	}

	if jsonStringBlockNumberTestnet != "" {
		ethBlock, err = strconv.ParseInt(strings.Replace(jsonDataBlockNumberTestnet.Result, "0x", "", -1), 16, 64)

		if err != nil {
			log.Print(err)
			up = 0
		} else {
			io.WriteString(w, formatValue("etherscan_block_number{network=\""+testNet+"\"}", "", integer64ToString(ethBlock)))
		}
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	log.Print("Serving /index")
	html := string(`<!doctype html>
<html>
    <head>
        <meta charset="utf-8">
        <title>Etherscan Exporter</title>
    </head>
    <body>
        <h1>Etherscan Exporter</h1>
        <p><a href="/metrics">Metrics</a></p>
    </body>
</html>
`)
	io.WriteString(w, html)
}

func main() {
	godotenv.Load()

	testMode = os.Getenv("TEST_MODE")
	if testMode == "1" {
		log.Print("Test mode is enabled")
	}

	accountIds = os.Getenv("ACCOUNTS")
	log.Print("Monitoring account id's: " + accountIds)

	apiKey = os.Getenv("API_KEY")

	testNet = os.Getenv("TESTNET")

	log.Print("Etherscan exporter listening on " + LISTEN_ADDRESS)
	http.HandleFunc("/", index)
	http.HandleFunc("/metrics", metrics)
	http.ListenAndServe(LISTEN_ADDRESS, nil)
}
