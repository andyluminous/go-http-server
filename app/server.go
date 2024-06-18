package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type request struct {
	method     string
	path       string
	path_steps []string
	headers    map[string]string
	content    []byte
}

var responseOkString = "HTTP/1.1 200 OK\r\n\r\n"
var responseCreatedString = "HTTP/1.1 201 Created\r\n\r\n"
var responseNotFoundString = "HTTP/1.1 404 Not Found\r\n\r\n"
var responseOkByteArray []byte = []byte(responseOkString)
var responseCreatedByteArray []byte = []byte(responseCreatedString)
var responseNotFoundByteArray []byte = []byte(responseNotFoundString)
var echoPathStart string = "echo"
var filesPathStart string = "files"
var directoryArg string = getDirectoryArg()

func getRequest(requestString string) request {
	requestRows := strings.Split(requestString, "\r\n")

	firstRowContents := strings.Split(requestRows[0], " ")

	startOfHeadersIndex := 0
	for i, row := range requestRows[1:] {
		if len(row) != 0 {
			startOfHeadersIndex = i + 1
			break
		}
	}
	endOfHeadersIndex := len(requestRows) - 1
	for i, row := range requestRows[startOfHeadersIndex:] {
		if len(row) == 0 {
			endOfHeadersIndex = i + startOfHeadersIndex
			break
		}
	}
	rawHeaders := requestRows[startOfHeadersIndex:endOfHeadersIndex]
	parsedHeaders := parseHeaders(rawHeaders)
	var requestContent []byte = nil
	if h, ok := parsedHeaders["Content-Length"]; ok {
		contentLength, _ := strconv.Atoi(h)
		body := []byte(strings.Join(requestRows[endOfHeadersIndex:], ""))
		requestContent = body[:contentLength]
	}

	return request{
		method:     firstRowContents[0],
		path:       firstRowContents[1],
		path_steps: getPathSteps(firstRowContents[1]),
		headers:    parsedHeaders,
		content:    requestContent,
	}
}

func getPathSteps(path string) []string {
	return strings.Split(strings.TrimSpace(path), "/")
}

func isEchoPath(path_steps []string) bool {
	return len(path_steps) > 1 && path_steps[1] == echoPathStart
}

func isReadFilesPath(req request) bool {
	return len(req.path_steps) > 1 && req.path_steps[1] == filesPathStart && req.method == "GET"
}

func isWriteFilesPath(req request) bool {
	return len(req.path_steps) > 1 && req.path_steps[1] == filesPathStart && req.method == "POST"
}

func getResponseWithContent(contentString string) []byte {
	responseString := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s\r\n", len(contentString), contentString)
	return []byte(responseString)
}

func getResponseWithFile(file []byte) []byte {
	responseString := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length: %d\r\n\r\n", len(file))
	resArray := []byte(responseString)
	res := append(resArray, file...)
	return res
}

func getEchoResponse(path_steps []string) []byte {
	echoString := strings.Join(path_steps[2:], "/")
	return getResponseWithContent(echoString)
}

func getReadFileResponse(path_steps []string) []byte {
	fileName := (path_steps[2])
	file, err := getFile(directoryArg, fileName)
	if err != nil {
		return responseNotFoundByteArray
	}
	return getResponseWithFile(file)
}

func getUserAgentResponse(headers map[string]string) []byte {
	if content, ok := headers["User-Agent"]; ok {
		return getResponseWithContent(content)
	}
	return make([]byte, 0)
}

func getDirectoryArg() string {
	args := os.Args
	for i := 0; i < len(args); i++ {
		if args[i] == "--directory" && len(args) > i+1 {
			return args[i+1]
		}
	}
	return ""
}

func getFile(folder string, fileName string) ([]byte, error) {
	if len(folder) == 0 {
		return make([]byte, 0), errors.New("Folder name is missing")
	}
	file, err := os.ReadFile(folder + "/" + fileName)
	if err != nil {
		return make([]byte, 0), err
	}

	return file, nil
}

func writeFile(folder string, req request) error {
	if len(folder) == 0 {
		return errors.New("folder name is missing")
	}
	file, err := os.Create(folder + "/" + req.path_steps[2])
	if err != nil {
		return err
	}
	defer file.Close()

	file.Write([]byte(req.content))
	return nil
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	buffer := make([]byte, 1024)
	_, err := conn.Read(buffer)

	if err != nil {
		fmt.Println("Read error: ", err.Error())
	}

	req := getRequest(string(buffer))

	var response []byte

	switch true {
	case req.path == "/":
		response = responseOkByteArray
	case isEchoPath(req.path_steps):
		response = getEchoResponse(req.path_steps)
	case isReadFilesPath(req):
		response = getReadFileResponse(req.path_steps)
	case isWriteFilesPath(req):
		writeFile(directoryArg, req)
		response = responseCreatedByteArray
	case req.path == "/user-agent":
		response = getUserAgentResponse(req.headers)
	default:
		response = responseNotFoundByteArray
	}

	_, err = conn.Write(response)

	if err != nil {
		fmt.Println("Error writing response", err.Error())
		os.Exit(1)
	}
}

func parseHeaders(rawHeaders []string) map[string]string {
	headers := make(map[string]string)
	for _, header := range rawHeaders {
		if len(header) == 0 || !strings.Contains(header, ":") {
			continue
		}
		splitHeader := strings.Split(header, ":")
		key := splitHeader[0]
		value := strings.TrimSpace(strings.Join(splitHeader[1:], ""))
		headers[key] = value
	}

	return headers
}

func main() {
	address := "0.0.0.0:4221"

	listener, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	defer listener.Close()

	fmt.Printf("Listening on: %s\n", address)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			os.Exit(1)
		}
		go handleConnection(conn)
	}
}
