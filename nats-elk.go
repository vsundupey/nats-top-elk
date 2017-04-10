// testrestcliant
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/jmcvetta/napping"
)

type Configuration struct {
	Interval    int
	LogStashUrl string
	LgLogin     string // logstash login
	LgPassword  string // logstash password
	NatsUrls    []string
}
type NatsNodeTopInfo struct {
	Varz  Varz
	Connz Connz
}
type Cluster struct {
	Addr         string
	Cluster_port int
}
type Varz struct {
	Server_id         string
	Host              string
	Addr              string
	Http_host         string
	Cluster           Cluster
	Start             string
	Now               time.Time
	Uptime            string
	Mem               float32
	Cpu               float32
	Connections       int
	Total_connections int
	Routes            int
	Remotes           int
	In_msgs           int
	Out_msgs          int
	In_bytes          int
	Out_bytes         int
	In_msgs_sec       int
	Out_msgs_sec      int
	In_bytes_sec      int
	Out_bytes_sec     int
	Slow_consumers    int
	Subscriptions     int
}
type Connection struct {
	Сid           int
	Ip            string
	Port          int
	Start         string
	Last_activity string
	Uptime        string
	Pending_bytes int
	In_msgs       int
	Out_msgs      int
	In_bytes      int
	Out_bytes     int
}
type Connz struct {
	Now             string
	Num_connections int
	Total           int
	Offset          int
	Limit           int
	Connections     []Connection
}
type PrevInOutValues struct {
	In_msgs   int
	Out_msgs  int
	In_bytes  int
	Out_bytes int

	Now time.Time
}
type InOutPerSec struct {
	In_msgs_sec   int
	Out_msgs_sec  int
	In_bytes_sec  int
	Out_bytes_sec int
}

var prev_vals map[string]*PrevInOutValues = make(map[string]*PrevInOutValues)

func main() {

	config := Configuration{}

	var configPath string

	flag.StringVar(&configPath, "c", "path to config file", "")
	flag.StringVar(&configPath, "string", "path to config file", "")

	setFlag(flag.CommandLine)
	flag.Parse()

	config = readConfig(configPath)

	httpClient := http.Client{}
	httpClient.Timeout = time.Duration(300) * time.Millisecond
	sessionToNats := napping.Session{Client: &httpClient}
	sessionToLogstash := napping.Session{Userinfo: url.UserPassword(config.LgLogin, config.LgPassword)}

	e := HttpError{}

	for true {
		for _, url := range config.NatsUrls {

			varz := Varz{}
			connzs := Connz{}
			natsNodeTopInfo := NatsNodeTopInfo{}

			varzUrl := url + "/varz"
			connzUrl := url + "/connz"

			varzResponse, err := sessionToNats.Get(varzUrl, nil, &varz, &e)

			if err != nil {
				fmt.Println(err)
				continue
			}

			connzResponse, err := sessionToNats.Get(connzUrl, nil, &connzs, &e)

			if err != nil {
				fmt.Println(err)
				continue
			}

			if varzResponse.Status() == 200 && connzResponse.Status() == 200 {

				perSecValues := getPerSecValues(url, varz)

				varz.In_bytes_sec = perSecValues.In_bytes_sec
				varz.Out_bytes_sec = perSecValues.Out_bytes_sec
				varz.In_msgs_sec = perSecValues.In_msgs_sec
				varz.Out_msgs_sec = perSecValues.Out_msgs_sec

				varz.Mem = varz.Mem / 1024 / 1024 // to MB
				natsNodeTopInfo.Varz = varz
				natsNodeTopInfo.Connz = connzs

				fmt.Println(natsNodeTopInfo)
				fmt.Printf("Sending to logstash -> ")
				logstashResponse, err := sessionToLogstash.Post(config.LogStashUrl, natsNodeTopInfo, nil, &e)

				if err != nil {
					fmt.Println(err)
				}
				if logstashResponse.Status() == 200 {
					fmt.Println("Success\n")
				}
			}
		}
		time.Sleep(time.Duration(config.Interval) * time.Millisecond)
	}
}
func getPerSecValues(url string, varz Varz) InOutPerSec {
	inOutPerSec := InOutPerSec{}

	if prev_vals[url] == nil {

		prev_vals[url] = &PrevInOutValues{}

		prev_vals[url].In_bytes = varz.In_bytes
		prev_vals[url].Out_bytes = varz.Out_bytes
		prev_vals[url].In_msgs = varz.In_msgs
		prev_vals[url].Out_msgs = varz.Out_msgs
		prev_vals[url].Now = varz.Now

		return InOutPerSec{}
	}

	// calculate
	in_bytes_delta := varz.In_bytes - prev_vals[url].In_bytes
	out_bytes_delta := varz.Out_bytes - prev_vals[url].Out_bytes

	in_msgs_delta := varz.In_msgs - prev_vals[url].In_msgs
	out_msgs_delta := varz.Out_msgs - prev_vals[url].Out_msgs

	sec := varz.Now.Second() - prev_vals[url].Now.Second()

	inOutPerSec.In_bytes_sec = in_bytes_delta / sec
	inOutPerSec.Out_bytes_sec = out_bytes_delta / sec
	inOutPerSec.In_msgs_sec = in_msgs_delta / sec
	inOutPerSec.Out_msgs_sec = out_msgs_delta / sec

	// save prev.values
	prev_vals[url].In_bytes = varz.In_bytes
	prev_vals[url].Out_bytes = varz.Out_bytes
	prev_vals[url].In_msgs = varz.In_msgs
	prev_vals[url].Out_msgs = varz.Out_msgs
	prev_vals[url].Now = varz.Now

	// return result
	return inOutPerSec
}
func readConfig(filepath string) Configuration {

	file, _ := os.Open(filepath)
	decoder := json.NewDecoder(file)
	configuration := Configuration{}
	err := decoder.Decode(&configuration)

	if err != nil {
		fmt.Println("error:", err)
	}

	return configuration
}

type HttpError struct {
	Message string
	Errors  []struct {
		Resource string
		Field    string
		Code     string
	}
}

func setFlag(flag *flag.FlagSet) {
	flag.Usage = func() {
		showHelp()
	}
}

func showHelp() {
	fmt.Println(`
Usage: CLI Template [OPTIONS]
Options:
    -c, --string     Path to config file.       
    -h, --help       prints this help info.
    `)
}