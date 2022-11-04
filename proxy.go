package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/go-redis/redis"
)

var IsDebug = false

// RedisTimeout caching timeout
const RedisTimeout = 24 * time.Hour

const XHTTPCachingRequestIDName = "X-HTTP-REQUEST-ID"

// soap action will be cached
// we get the action name from the request header "Soapaction"
// maybe you can modify it for your use case
var allowedCachingSOAPActions = []string{"getList", "call", "getMessage"}

var tobeHonoredHeaderAttrs = []string{"x-connector-entity", "accept", "user-agent", "Content-Length", "Accept-Encoding", "Accept-Language", "Soapaction"}

func orPanic(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	argsWithoutProg := os.Args[:]
	for _, argWithoutProg := range argsWithoutProg {
		if strings.TrimRight(argWithoutProg, "\n") == "-d" || strings.TrimRight(argWithoutProg, "\n") == "--debug" {
			IsDebug = true
		}
	}

	if IsDebug {
		fmt.Println("DEBUG mode ENABLED!")
	} else {
		fmt.Println("DEBUG mode DISABLED!")
	}

	redisClient, err := newRedisClient()
	orPanic(err)

	proxy := goproxy.NewProxyHttpServer()
	proxy.OnRequest().DoFunc(

		func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
			startReqFn()
			if !isToBeCached(req) {
				fmt.Println("Not a to-be-cached request. Forwarding!")
				dump, err := httputil.DumpRequestOut(req, true)
				debug(dump, err)
				endReqFn()
				return req, nil
			}

			dump, err := httputil.DumpRequestOut(req, true)
			redisKey, _, isOkay := requestToRedisKey(req)
			if !isOkay {
				endReqFn()
				return req, nil
			}

			injectCachingIDToRequest(req, redisKey)
			debugReq(dump, err)
			cachedResp, err := redisClient.Get(redisKey).Bytes()
			if err != nil {
				fmt.Println("Not found key " + redisKey + " in Redis!!!")
			} else {
				resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(cachedResp)), nil)
				orPanic(err)
				fmt.Println(redisKey + " found in Redis!!!")
				endReqFn()
				return req, resp
			}

			endReqFn()
			return req, nil
		})

	proxy.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		req := resp.Request
		if resp == nil || resp.Request == nil || !isToBeCached(req) {
			return resp
		}

		startRespFn()
		redisKey := req.Header.Get(XHTTPCachingRequestIDName)
		if redisKey == "" {
			fmt.Println("Cannot get request caching ID")
			endRespFn()
			return resp
		}

		respToBytes, err := httputil.DumpResponse(resp, true)
		debugResp(respToBytes, err)
		orPanic(err)
		cachedResp, err := redisClient.Get(redisKey).Bytes()
		_ = cachedResp
		if err == redis.Nil || err == nil {
			setErr := redisClient.Set(redisKey, respToBytes, RedisTimeout).Err()
			orPanic(setErr)
			fmt.Println("Set key " + redisKey + " to Redis!")
		}

		endRespFn()
		return resp
	})

	verbose := flag.Bool("v", false, "should every proxy request be logged to stdout")
	addr := flag.String("addr", ":48080", "proxy listen address")
	flag.Parse()
	proxy.Verbose = *verbose
	fmt.Println("Start HTTP Caching Proxy on address 127.0.0.1" + *addr)
	log.Fatal(http.ListenAndServe(*addr, proxy))

}

func injectCachingIDToRequest(req *http.Request, cachingID string) {
	req.Header.Set(XHTTPCachingRequestIDName, cachingID)
	fmt.Println("Inject " + XHTTPCachingRequestIDName + ":" + cachingID + " to request header")
}

func startReqFn() {
	fmt.Printf("\n\nSTART REQUEST--------------------------\n")
}

func endReqFn() {
	fmt.Printf("\n----------------------------END REQUEST\n\n")
}

func startRespFn() {
	fmt.Printf("\n\nSTART RESPONSE--------------------------\n")
}

func endRespFn() {
	fmt.Printf("\n----------------------------END RESPONSE\n\n")
}

func isToBeCached(req *http.Request) bool {
	if "GET" == req.Method || ("POST" == req.Method && isAllowedCachingSOAPAction(req)) {
		return true
	}

	return false
}

func isAllowedCachingSOAPAction(req *http.Request) bool {
	soapAction := req.Header.Get("Soapaction")
	soapAction = strings.Replace(soapAction, "\"", "", -1)
	if contains(allowedCachingSOAPActions, soapAction) {
		fmt.Println(fmt.Sprintf("SOAP action %v is allowed to be cached", soapAction))
		return true
	}

	fmt.Println(fmt.Sprintf("SOAP action %v is NOT allowed to be cached", soapAction))
	return false
}

func contains(stringSlice []string, searchString string) bool {
	for _, value := range stringSlice {
		if value == searchString {
			return true
		}
	}
	return false
}

func requestToRedisKey(req *http.Request) (string, string, bool) {
	md5Value, isOkay := requestToMD5(req)
	return fmt.Sprintf("%x", md5Value), fmt.Sprintf("%v_%v_%v_%x", req.Method, req.Proto, req.URL, md5Value), isOkay
}

func requestToMD5(req *http.Request) ([16]byte, bool) {
	var isOk = true
	var request []string

	request = append(request, fmt.Sprintf("URL: %v\nMethod: %v\nProtocol: %v", req.URL, req.Method, req.Proto))
	for _, name := range tobeHonoredHeaderAttrs {
		request = append(request, fmt.Sprintf("%v: %v", name, req.Header.Get(name)))
	}

	if "POST" == req.Method {
		var bodyBytes []byte
		var err error
		if req.Body != nil {
			bodyBytes, err = io.ReadAll(req.Body)
			if err != nil {
				orPanic(err)
				isOk = false
			}
		}
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		bodyString := string(bodyBytes)
		request = append(request, fmt.Sprintf("%s", bodyString))
	}

	joined := strings.Join(request, "\n")
	debugStr("MD5 START----------------", nil)
	debugStr(joined, nil)
	hashed := md5.Sum([]byte(joined))
	debugStr(fmt.Sprintf("MD5: %x", hashed), nil)
	debugStr("MD5 END----------------", nil)

	return hashed, isOk
}

func debugStr(data string, err error) {
	debug([]byte(data), err)
}

func debug(data []byte, err error) {
	if !IsDebug {
		return
	}

	if err == nil {
		fmt.Printf("[DEBUG] %s\n", data)
	} else {
		orPanic(err)
	}
}

func debugReq(data []byte, err error) {
	if !IsDebug {
		return
	}

	if err == nil {
		fmt.Printf("[DEBUG] ----------REQUEST-----------\n%s\n\n", data)
	} else {
		orPanic(err)
	}
}

func debugResp(data []byte, err error) {
	if !IsDebug {
		return
	}

	if err == nil {
		fmt.Printf("[DEBUG] ----------RESPONSE-----------\n%s\n\n", data)
	} else {
		orPanic(err)
	}
}

func newRedisClient() (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "",
		DB:       0,
	})

	pong, err := client.Ping().Result()
	orPanic(err)
	fmt.Println("Ping ---> Server response ", pong)

	return client, err
}
