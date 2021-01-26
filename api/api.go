package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/gin-gonic/gin"
)

var ginLambda *ginadapter.GinLambda

type binaryFileSystem struct {
	fs http.FileSystem
}

func (b *binaryFileSystem) Open(name string) (http.File, error) {
	return b.fs.Open(name)
}

func (b *binaryFileSystem) Exists(prefix, filepath string) bool {
	if p := strings.TrimPrefix(filepath, prefix); len(p) > len(filepath) {
		if _, err := b.fs.Open(p); err != nil {
			return false
		}
		return true
	}

	return false
}

// BinaryFileSystem ...
func BinaryFileSystem(root string) *binaryFileSystem {
	return &binaryFileSystem{
		fs: &assetfs.AssetFS{
			Asset:     Asset,
			AssetDir:  AssetDir,
			AssetInfo: AssetInfo,
			Prefix:    root,
			Fallback:  "index.html",
		},
	}
}

func init() {
	// stdout and stderr are sent to AWS CloudWatch Logs
	log.Printf("Gin cold start")
	r := gin.Default()

	api := r.Group("/api")
	{
		// Serve frontend static files
		api.StaticFS("/ui", BinaryFileSystem("client/build"))

		api.GET("/validate", func(c *gin.Context) {
			data, result := validateRedirectQS(c.Request.URL.Query())
			if result != nil {
				c.JSON(http.StatusOK, result)
				return
			}

			asset, err := Asset("client/build/index.html")
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"message": err.Error(),
				})
				return
			}

			tmpl, err := template.New("").Parse(string(asset))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"message": err.Error(),
				})
				return
			}

			buf := new(bytes.Buffer)
			defer buf.Reset()

			err = tmpl.Execute(buf, data)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": err.Error(),
				})
				return
			}

			c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
		})

		api.GET("/ping", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "pong",
			})
		})

	}

	ginLambda = ginadapter.New(r)
}

var (
	ErrDataCorrupt   = errors.New("data is corrupt")
	ErrNoRedirectURL = errors.New("param of redirect_url is empty or invalid")
)

// ErrorResponse is wrap error response
type ErrorResponse struct {
	Error ErrorResponseData `json:"error,omitempty"`
}

// ErrorResponseData is wrap data of error
type ErrorResponseData struct {
	Message string `json:"message,omitempty"`
}

func apiResponse(statusCode int, data interface{}) (events.APIGatewayProxyResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		errResponse := ErrorResponse{
			Error: ErrorResponseData{
				Message: "internal server error",
			},
		}

		body, _ := json.Marshal(errResponse)
		return events.APIGatewayProxyResponse{
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body:       string(body),
			StatusCode: http.StatusInternalServerError,
		}, nil

	}

	return events.APIGatewayProxyResponse{
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body:       string(body),
		StatusCode: statusCode,
	}, nil

}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	/*
		data, result := validateRedirectQS(request.QueryStringParameters)
		if result != nil {
			return apiResponse(http.StatusOK, result)
		}

		return renderHTML(data)
	*/
	return ginLambda.ProxyWithContext(ctx, request)
}

const (
	nameQueryString          = "name"
	redirectURLQueryString   = "redirect_url"
	scopeQueryString         = "scope"
	consentPeriodQueryString = "consent_period"
	productIDQueryString     = "product_id"
	productQTYQueryString    = "product_qty"
)

type errorTransaction struct {
	code    string
	message string
}

type errorTransactions []errorTransaction

func availableValidate() errorTransactions {
	return errorTransactions{
		{nameQueryString, "param of name is not found"},
		{redirectURLQueryString, "param of redirect_url not found"},
		{scopeQueryString, "param of scope is not found"},
		{consentPeriodQueryString, "param of consent_period is not found"},
		{productIDQueryString, "param of product_id is not found"},
		{productQTYQueryString, "param product_qty is not found"},
	}
}

func errResponse(msg string) *ErrorResponse {
	return &ErrorResponse{
		Error: ErrorResponseData{
			Message: msg,
		},
	}
}

const (
	ErrValueEmpty      = "error param of %v is empty"
	ErrValueInvalid    = "error param of %v invalid value"
	ErrValueSufficient = "error param of %v is not sufficient"
)

type ResponseData map[string]interface{}

func validateRedirectQS(queryString map[string][]string) (ResponseData, *ErrorResponse) {
	var responseData = ResponseData{}
	for _, param := range availableValidate() {
		v, ok := queryString[param.code]
		if !ok {
			return nil, errResponse(param.message)
		}

		if v[0] == "" {
			return nil, errResponse(fmt.Sprintf(ErrValueEmpty, param.code))
		}

		switch param.code {
		case productQTYQueryString:
			value, err := strconv.Atoi(v[0])
			if err != nil {
				return nil, errResponse(fmt.Sprintf(ErrValueInvalid, param.code))
			}

			if value < 1 {
				return nil, errResponse("The requested qty is not available")
				//		return errResponse("Sorry, we do not have enough products in stock")
			}
		}

		responseData[param.code] = v
	}

	return responseData, nil
}
