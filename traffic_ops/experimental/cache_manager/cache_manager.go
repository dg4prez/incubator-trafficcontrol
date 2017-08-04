package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	to "github.com/apache/incubator-trafficcontrol/traffic_ops/client"
)

const (
	// LogLocationStdout indicates the stdout IO stream
	LogLocationStdout = "stdout"
	// LogLocationStderr indicates the stderr IO stream
	LogLocationStderr = "stderr"
	// LogLocationNull indicates the null IO stream (/dev/null)
	LogLocationNull = "null"
	// DefaultConfig sets the default configuration file location
	DefaultConfig = "/opt/cache_manager/etc/cache_manager.config"
	//Version sets the CacheManager version
	Version = "0.1"
	//UserAgent sets the CacheManager UA
	UserAgent = "cache_manager/" + Version
	// ClientTimeout duration
	ClientTimeout = time.Duration(10 * time.Second)
)

//Config : contains configuration information to be used during runtime.
type Config struct {
	// RFCCompliant determines whether `Cache-Control: no-cache` requests are honored. The ability to ignore `no-cache` is necessary to protect origin servers from DDOS attacks. In general, CDNs and caching proxies with the goal of origin protection should set RFCComplaint false. Cache with other goals (performance, load balancing, etc) should set RFCCompliant true.
	RFCCompliant       bool `json:"rfc_compliant"`
	Dispersion         int
	ValidateInterval   int
	SyncInterval       int
	Retries            int
	WaitForParents     bool
	ToURL              string `json:"to_url"`
	ToCacheURL         string
	Username           string
	Password           string
	LogLocationError   string `json:"log_location_error"`
	LogLocationWarning string `json:"log_location_warning"`
	LogLocationInfo    string `json:"log_location_info"`
	LogLocationDebug   string `json:"log_location_debug"`
	LogLocationEvent   string `json:"log_location_event"`
}

// MyConfig is the program-wide config.
var MyConfig = Config{
	RFCCompliant:       true,
	Dispersion:         300,
	ValidateInterval:   60,
	SyncInterval:       300,
	Retries:            3,
	WaitForParents:     false,
	LogLocationError:   LogLocationStderr,
	LogLocationWarning: LogLocationStderr,
	LogLocationInfo:    LogLocationStderr,
	LogLocationDebug:   LogLocationStderr,
	LogLocationEvent:   LogLocationStderr,
}

func main() {

	hostname, error := os.Hostname()
	if error != nil {
		panic(error)
	}

	shortHostname := strings.Split(hostname, ".")[0]

	// testing: change shortHostName to arbitrary cache name
	shortHostname = "odol-atsec-atl-01"

	t := time.Now()
	fmt.Printf("Starting Cache Manager Service at %[1]v on %[2]v...\n", t.Format("2006-01-02 15:04:05 MST"), shortHostname)

	configFileName := flag.String("config", "", "The config file path")
	flag.Parse()

	if *configFileName == "" {
		fmt.Printf("Error starting service: The --config argument is required\n")
		os.Exit(1)
	}

	if err := LoadConfig(*configFileName); err != nil {
		fmt.Printf("Error starting service: loading config: %v\n", err)
		os.Exit(1)
	}

	if err := configCheck(); err != nil {
		fmt.Printf("Configuration error: %v\n", err)
		os.Exit(1)
	}

	toInsecure := true
	toClient, err := to.LoginWithAgent(MyConfig.ToURL, MyConfig.Username, MyConfig.Password, toInsecure, UserAgent, toInsecure, ClientTimeout)
	if err != nil {
		fmt.Printf("Error connecting to Traffic Ops: %v\n", err)
		os.Exit(1)
	}
	//fmt.Println(toClient)
	//deliveryService, err := toClient.DeliveryService("10243")
	//deliveryServices, err := toClient.DeliveryServices()
	//fmt.Println(deliveryService)
	//fmt.Println(deliveryServices[3])

	server, err := toClient.GetServerMetadata(shortHostname)

	if err != nil {
		fmt.Printf("Error getting metadata: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf(server.HostName)
	fmt.Println()
	for _, ds := range server.DSList {
		fmt.Println(ds.XMLID)
	}
	/*for i, ds := range deliveryServices {
		if i == 3 {
			fmt.Println(ds)
		}
	}*/

}

// LoadConfig loads the given config file. If an empty string is passed, the default config is returned.
func LoadConfig(fileName string) error {
	if fileName == "" {
		return nil
	}
	configBytes, err := ioutil.ReadFile(fileName)
	if err == nil {
		err = json.Unmarshal(configBytes, &MyConfig)
	}
	return err
}

func configCheck() error {
	var err error
	if MyConfig.ToURL == "" {
		err = errors.New("Invalid URL provided for Traffic Ops")
	}

	if MyConfig.Username == "" || MyConfig.Password == "" {
		err = errors.New("Username and password cannot be blank")
	}
	if err == nil {
	}
	return err
}
