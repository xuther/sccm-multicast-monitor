package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type config struct {
	FullRegex         string
	NamespaceRegex    string
	ObjectRegex       string
	PostAddress       string
	PostClientAddress string
}

type namespace struct {
	Values    map[string]string
	Clients   []map[string]string
	TimeStamp time.Time
}

func importConfig(configPath string) (config, error) {

	var configurationData config

	log.Printf("Importing the configuration information from %v\n", configPath)

	f, err := ioutil.ReadFile(configPath)
	if err != nil {
		return config{}, err
	}

	err = json.Unmarshal(f, &configurationData)
	if err != nil {
		return configurationData, err
	}

	log.Printf("\n%+v\n", configurationData)

	return configurationData, nil
}

func readFile(fileLocation string) (string, error) {
	b, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func parseFile(fileLocation string) ([]namespace, error) {
	var toReturn []namespace

	value, err := readFile(fileLocation)
	if err != nil {
		return toReturn, err
	}

	if debug {
		log.Printf("Value read in from file: %s\n", value)
	}

	fullRegex := regexp.MustCompile(`Namespace\s*[\n\r]*-+[\n\r]+?(Name[\s\S]+?Total Number of Clients Connected to Namespace: [0-9]+[\n\r]+?)`)
	namespaceRegex := regexp.MustCompile(`(Name[\s\S]*?)[\n\r]+?[\r]+?`)
	clientRegex := regexp.MustCompile(`(ClientId[\s\S]*?Network .+)`)

	matches := fullRegex.FindAllStringSubmatch(value, -1)

	if debug {
		log.Printf("Matches found: %+v\n", matches)
	}

	for _, v := range matches {
		namespaceMatch := namespaceRegex.FindString(v[1])
		if debug {
			log.Printf("Namespace found: %+v\n", namespaceMatch)
		}

		values := strings.Split(v[1], "*******************************************************************************")
		if debug {
			log.Printf("Strings split: %+v\n", values)
		}

		var clientMaps []map[string]string
		for _, v2 := range values {
			client := clientRegex.FindString(v2)
			if len(client) < 1 {
				continue
			}

			if debug {
				log.Printf("Found Client: %+v\n", client)
			}

			clientValues := strings.Split(client, "\n")

			clientMap := make(map[string]string)

			for _, v3 := range clientValues {
				keyValue := strings.SplitN(v3, ":", 2)

				clientMap[keyValue[0]] = keyValue[1]
			}
			clientMaps = append(clientMaps, clientMap)
		}

		namespaceValues := strings.Split(namespaceMatch, "\n")
		namespaceMap := make(map[string]string)

		for _, v2 := range namespaceValues {
			keyValue := strings.SplitN(v2, ":", 2)
			if len(keyValue) < 2 {
				continue
			}
			namespaceMap[keyValue[0]] = keyValue[1]
		}
		if len(clientMaps) < 1 {
			continue
		}
		toReturn = append(toReturn, namespace{Values: namespaceMap, Clients: clientMaps, TimeStamp: time.Now()})
	}

	return toReturn, nil
}

var configuration config

func postToSearch(b []byte, address string) error {
	log.Printf("Post address: %s\n", configuration.PostAddress)
	resp, err := http.Post(address, "application/json", bytes.NewReader(b))

	log.Printf("%s\n", b)

	if err != nil {
		log.Printf("Error: %s\n", err.Error())
		panic(err)
	}

	by, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		panic(err)
	}
	log.Printf("Response: %s\n", by)

	return nil
}

var debug = false

func main() {
	var ConfigFileLocation = flag.String("config", "./config.json", "The locaton of the config file.")
	var FileLocation = flag.String("file", "./info.txt", "The location of the information to parse")
	var Debug = flag.Bool("debug", false, "PrintDebuggingInfo true or false")
	flag.Parse()

	log.SetOutput(os.Stdout)

	debug = *Debug

	if debug {
		log.Printf("Debugging turned on.\n")
	}

	c, err := importConfig(*ConfigFileLocation)
	if err != nil {
		panic(err)
	}
	configuration = c
	//log.Printf("Configuration: %+v\n", configuration)
	values, err := parseFile(*FileLocation)

	for _, namespace := range values {
		b, err := json.Marshal(&namespace)
		if err != nil {
			panic(err)
		}
		log.Print("Sending Namespace to ES\n", b)
		err = postToSearch(b, configuration.PostAddress)
		log.Print("Sending Clients to ES\n", b)
		for _, client := range namespace.Clients {
			client["Timestamp"] = time.Now().String()
			b, err := json.Marshal(&client)
			if err != nil {
				panic(err)
			}

			err = postToSearch(b, configuration.PostClientAddress)
		}
	}
	if err != nil {
		panic(err)
	}
}
