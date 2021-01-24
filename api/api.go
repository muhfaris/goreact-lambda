package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	ginadapter "github.com/awslabs/aws-lambda-go-api-proxy/gin"
	"github.com/gin-gonic/contrib/renders/multitemplate"
	"github.com/gin-gonic/gin"
)

var ginLambda *ginadapter.GinLambda

func LoadTemplates(paths ...string) *template.Template {
	var err error
	var tpl *template.Template
	var path string
	var data []byte
	for _, path = range paths {
		data, err = Asset("client/build/" + path)
		if err != nil {
			fmt.Println(err)
		}

		var tmp *template.Template
		if tpl == nil {
			tpl = template.New(path)
		}

		if path == tpl.Name() {
			tmp = tpl
		} else {
			tmp = tpl.New(path)
		}

		_, err = tmp.Parse(string(data))
		if err != nil {
			fmt.Println(err)
		}
	}

	return tpl
}

func Init() {
	// stdout and stderr are sent to AWS CloudWatch Logs
	log.Printf("Gin cold start")
	r := gin.Default()

	var render multitemplate.Render
	render = multitemplate.New()
	render.Add("index", LoadTemplates("index.html"))
	r.HTMLRender = render

	// Serve frontend static files
	//r.Use(static.Serve("/static", static.LocalFile("./client/build/static", true)))
	//r.StaticFS("/static", http.Dir("./client/build/static"))

	// Serves the "static" directory's files from binary data.
	// You have to pass the "Asset" function generated by
	// go-bindata (https://github.com/jteeuwen/go-bindata).
	/*
		r.Use(staticbin.Static(Asset, staticbin.Options{
			// Dir prefix will be trimmed. It needs to separate namespace.
			Dir: "/static",
		}))
	*/

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	//Serve static via bindata and handle via react app
	//in case when static file was not found
	r.GET("/api", func(c *gin.Context) {
		/*
			_, result := validateRedirectQS(c.Request.URL.Query())
			if result != nil {
				c.JSON(http.StatusOK, result)
				return
			}
		*/

		c.HTML(200, "index", gin.H{
			"Title": "Go-Test",
		})
	})

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
	data, result := validateRedirectQS(request.QueryStringParameters)
	if result != nil {
		return apiResponse(http.StatusOK, result)
	}

	return renderHTML(data)
	//return ginLambda.ProxyWithContext(ctx, request)
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

func validateRedirectQS(queryString map[string]string) (ResponseData, *ErrorResponse) {
	var responseData = ResponseData{}
	for _, param := range availableValidate() {
		v, ok := queryString[param.code]
		if !ok {
			return nil, errResponse(param.message)
		}

		if v == "" {
			return nil, errResponse(fmt.Sprintf(ErrValueEmpty, param.code))
		}

		switch param.code {
		case productQTYQueryString:
			value, err := strconv.Atoi(v)
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