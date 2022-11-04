# Goals
***This is a Go proxy used to caching HTTP request.***  
&#9989;&nbsp;&nbsp;_Forwarding request._  
&#9989;&nbsp;&nbsp;_Cache response base on custom header attribute._  
&#10060;&nbsp;&nbsp;_Enable/Disable caching_  
&#10060;&nbsp;&nbsp;_File logging_  

# Setup DE

## 1. Install Go lang and project dependencies.

**Go lang**  
```
brew install go 
 ```  
**Project dependencies**  
``` 
go get -u github.com/elazarl/goproxy 
go get -u github.com/go-redis/redis 
```
**[Optional] Atom and go-plus**  

## 2. Redis.

**Install Redis package**  
``` 
brew install redis 
```  
**Install Redis as OSX service**  
``` 
brew services start redis 
```  
**Connect to redis server to make sure it was start successfully**  
``` 
redis-cli 
```  
Ping Redis server  
``` 
redis 127.0.0.1:6379> ping 
```  
If Redis server responses _PONG_, your server was up and running.  

# Deployment
**Start your proxy by**  
```go 
go run proxy.go 
```
Your proxy will be start at *:48080  

**Configure your application server with JVM proxy options:**  
_-Dhttp.proxyHost=&lt;proxyHostName&gt;_  
_-Dhttp.proxyPort=&lt;proxyPortNumber&gt;_  
_-Dhttps.proxyHost=&lt;secureProxyHostName&gt;_  
_-Dhttps.proxyPort=&lt;secureProxyHostName&gt;_  

**Useful Redis commands**  
**NOTES:** You should connect to Redis server using _redis-cli_  
_Flush all data_  
``` flushall ```  
_Check all keys(urls)_  
``` keys * ```  
_Check if a key(url) exists_  
``` get <key> ```  

# Resouces
[Go lang](https://golang.org/)  
[Redis](https://redis.io/topics/introduction)  
[Go proxy](https://github.com/elazarl/goproxy)  
[Go and Redis](https://github.com/go-redis/redis)  
