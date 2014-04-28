/*
* ytgate.go
* Find fast domain
* 
* Chapman Ou <ochapman@ochapman.cn>
* Sat Apr 26 21:32:17 CST 2014
*/
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type Zone struct {
	Name  string
	Count int
}

type PDuration struct {
	min    time.Duration
	avg    time.Duration
	max    time.Duration
	stddev time.Duration
}

type PingDuration struct {
	Name string
	Time PDuration
}

type ByAvgDuration []PingDuration

func (a ByAvgDuration) Len() int           { return len(a) }
func (a ByAvgDuration) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByAvgDuration) Less(i, j int) bool { return a[i].Time.avg < a[j].Time.avg }

// parse round-trip line
// Sample: round-trip min/avg/max/stddev = 334.356/383.656/438.647/42.766 ms
func parse(s string) PDuration {
	var pd PDuration
	sub := strings.Split(s, " ")
	unit := sub[4]
	t := strings.Split(sub[3], "/")
	pd.min, _ = time.ParseDuration(t[0] + unit)
	pd.avg, _ = time.ParseDuration(t[1] + unit)
	pd.max, _ = time.ParseDuration(t[2] + unit)
	pd.stddev, _ = time.ParseDuration(t[3] + unit)
	return pd
}

func getPDuration(out []byte) (PDuration, error) {
	re, err := regexp.Compile(`round-trip.*`)
	if err != nil {
		return PDuration{}, err
	}
	smatch := re.FindStringSubmatch(string(out))
	if len(smatch) == 0 {
		return PDuration{}, errors.New("Not found time")
	}
	pd := parse(smatch[0])
	return pd, nil
}

func doPing(addr string) ([]byte, error) {
	out, err := exec.Command("/sbin/ping", "-c 10", "-t 10", addr).CombinedOutput()
	if err != nil {
		return out, err
	}
	return out, nil
}

func ping(addr string) (PingDuration, error) {
	var pingd PingDuration
	out, err := doPing(addr)
	if err != nil {
		return pingd, err
	}
	pd, err := getPDuration(out)
	if err != nil {
		return pingd, err
	}
	pingd.Name = addr
	pingd.Time = pd
	//fmt.Println(pingd.Name, pingd.Time.avg)
	return pingd, nil
}

func getPingDurations(ch <-chan PingDuration, done <-chan bool) []PingDuration {
	pingds := make([]PingDuration, 0)
	select {
	case p, ok := <-ch:
		if ok {
			pingds = append(pingds, p)
		} else {
			return pingds
		}
	case <-done:
		return pingds
	}
	return pingds
}

func printPingDurations(pingds []PingDuration) {
	for _, p := range pingds {
		fmt.Println(p.Name, p.Time.avg)
	}
}

func PingYT() (<-chan PingDuration, <-chan bool) {
	done := make(chan bool)
	addrs := getYtgateServerAddr()
	chpingd := make(chan PingDuration)
	var wg sync.WaitGroup
	wg.Add(len(addrs))
	for _, a := range addrs {
		go func(addr string) {
			pingd, _ := ping(addr)
			chpingd <- pingd
			wg.Done()
		}(a)
	}
	go func() {
		wg.Wait()
		done <- true
	}()
	return chpingd, done
}

func SortYT(pingds []PingDuration) {
	sort.Sort(ByAvgDuration(pingds))
	printPingDurations(pingds)
}

func getYtgateServerAddr() []string {
	MainDomainName := "vpncute.com"
	Zones := []Zone{
		{Name: "sg", Count: 2},
		{Name: "jp", Count: 3},
		{Name: "us", Count: 5},
		{Name: "tw", Count: 1},
		{Name: "hk", Count: 2},
		{Name: "uk", Count: 1}}

	addrs := make([]string, 0)
	for _, z := range Zones {
		for i := 1; i <= z.Count; i++ {
			addr := fmt.Sprintf("%s%d.%s", z.Name, i, MainDomainName)
			addrs = append(addrs, addr)
		}
	}
	return addrs
}

func main() {
	pingds := make([]PingDuration, 0)
	chpingd, done := PingYT()
	for {
		select {
		case <-done:
			SortYT(pingds)
			os.Exit(0)
		case ch := <-chpingd:
			pingds = append(pingds, ch)
		}
	}
}
