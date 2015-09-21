package api

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"strings"
)

func httpGet(url string, json bool, userAgent string) (string, int, error) {
	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", 0, err
	}
	if json {
		r.Header.Set("Accept", "application/json")
	}
	r.Header.Set("User-Agent", userAgent)
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return "", 0, err
	}
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", 0, err
	}
	return string(data), res.StatusCode, nil
}

func TestGetIP(t *testing.T) {
	log.SetOutput(ioutil.Discard)
	toJSON := func(k string, v string) string {
		return fmt.Sprintf("{\n  \"%s\": \"%s\"\n}", k, v)
	}
	s := httptest.NewServer(New().Handlers())
	host,err := net.LookupAddr("127.0.0.1")
	var hostname string
	if err != nil {
	    hostname = ""
	} else {
	    hostname = strings.Join(host,", ")
	}
	jsonAll := "{\n  \"Accept-Encoding\": [\n    \"gzip\"\n  ]," +
		"\n  \"X-Ifconfig-Country\": [\n    \"\"\n  ]," +
		"\n  \"X-Ifconfig-Hostname\": [\n    \"" + hostname + "\"\n  ]," +
		"\n  \"X-Ifconfig-Ip\": [\n    \"127.0.0.1\"\n  ]\n}"

	var tests = []struct {
		url       string
		json      bool
		out       string
		userAgent string
		status    int
	}{
		{s.URL, false, "127.0.0.1\n", "curl/7.26.0", 200},
		{s.URL, false, "127.0.0.1\n", "Wget/1.13.4 (linux-gnu)", 200},
		{s.URL, false, "127.0.0.1\n", "fetch libfetch/2.0", 200},
		{s.URL + "/x-ifconfig-ip.json", false, toJSON("x-ifconfig-ip", "127.0.0.1"), "", 200},
		{s.URL, true, toJSON("x-ifconfig-ip", "127.0.0.1"), "", 200},
		{s.URL + "/foo", false, "no value found for: foo", "curl/7.26.0", 404},
		{s.URL + "/foo", true, "{\n  \"error\": \"no value found for: foo\"\n}", "curl/7.26.0", 404},
		{s.URL + "/all.json", false, jsonAll, "", 200},
	}

	for _, tt := range tests {
		out, status, err := httpGet(tt.url, tt.json, tt.userAgent)
		if err != nil {
			t.Fatal(err)
		}
		if status != tt.status {
			t.Errorf("Expected %d, got %d", tt.status, status)
		}
		if out != tt.out {
			t.Errorf("Expected %q, got %q", tt.out, out)
		}
	}
}

func TestIPFromRequest(t *testing.T) {
	var tests = []struct {
		in  *http.Request
		out net.IP
	}{
		{&http.Request{RemoteAddr: "1.3.3.7:9999"}, net.ParseIP("1.3.3.7")},
		{&http.Request{Header: http.Header{"X-Real-Ip": []string{"1.3.3.7"}}}, net.ParseIP("1.3.3.7")},
	}
	for _, tt := range tests {
		ip, err := ipFromRequest(tt.in)
		if err != nil {
			t.Fatal(err)
		}
		if !ip.Equal(tt.out) {
			t.Errorf("Expected %s, got %s", tt.out, ip)
		}
	}
}

func TestCmdFromParameters(t *testing.T) {
	var tests = []struct {
		in  url.Values
		out Cmd
	}{
		{url.Values{}, Cmd{Name: "curl"}},
		{url.Values{"cmd": []string{"foo"}}, Cmd{Name: "curl"}},
		{url.Values{"cmd": []string{"curl"}}, Cmd{Name: "curl"}},
		{url.Values{"cmd": []string{"fetch"}}, Cmd{Name: "fetch", Args: "-qo -"}},
		{url.Values{"cmd": []string{"wget"}}, Cmd{Name: "wget", Args: "-qO -"}},
	}
	for _, tt := range tests {
		cmd := cmdFromQueryParams(tt.in)
		if !reflect.DeepEqual(cmd, tt.out) {
			t.Errorf("Expected %+v, got %+v", tt.out, cmd)
		}
	}
}

func TestCLIMatcher(t *testing.T) {
	browserUserAgent := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_8_4) " +
		"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/30.0.1599.28 " +
		"Safari/537.36"
	var tests = []struct {
		in  string
		out bool
	}{
		{"curl/7.26.0", true},
		{"Wget/1.13.4 (linux-gnu)", true},
		{"fetch libfetch/2.0", true},
		{browserUserAgent, false},
	}
	for _, tt := range tests {
		r := &http.Request{Header: http.Header{"User-Agent": []string{tt.in}}}
		if got := cliMatcher(r, nil); got != tt.out {
			t.Errorf("Expected %t, got %t for %q", tt.out, got, tt.in)
		}
	}
}
