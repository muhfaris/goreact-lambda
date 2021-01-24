package main

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/thedevsaddam/renderer"
)

var (
	ErrNoViewHTML = errors.New("error index html is empty")
	rnd           *renderer.Render
)

// GetRootSchema is to get root graphql schema
func GetRootSchema() string {
	buf := bytes.Buffer{}
	for _, name := range AssetNames() {
		b := MustAsset(name)
		buf.Write(b)

		if len(b) > 0 && b[len(b)-1] != '\n' {
			buf.WriteByte('\n')
		}
	}

	return buf.String()
}

func renderHTML(data map[string]interface{}) (events.APIGatewayProxyResponse, error) {
	assets := GetRootSchema()
	var tmpl, err = template.New("").Parse(assets)
	if err != nil {
		return apiResponse(http.StatusInternalServerError, errResponse(err.Error()))
	}

	buf := new(bytes.Buffer)
	defer buf.Reset()

	err = tmpl.Execute(buf, data)
	if err != nil {
		return apiResponse(http.StatusInternalServerError, errResponse(err.Error()))
	}

	return events.APIGatewayProxyResponse{
		Headers: map[string]string{
			"Content-Type": "text/html; charset=UTF-8",
		},
		Body:       buf.String(),
		StatusCode: 200,
	}, nil
}
