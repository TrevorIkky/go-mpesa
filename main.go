package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const businessShortCode = 600982
const commandId = "CustomerBuyGoodsOnline"
const url = "https://sandbox.safaricom.co.ke/mpesa/c2b/v1/simulate"

type InvalidPhonenumber struct {
	message string
}

func (e InvalidPhonenumber) Error() string {
	return fmt.Sprintf("The phonenumber provided is invalid. %s", e.message)
}

type push struct {
	Phone  string `json:"phone"`
	Amount string `json:"amount"`
}

type C2BRequest struct {
	ShortCode     int
	CommandID     string
	Amount        string
	Msisdn        string
	BillRefNumber string // Fill for paybill
}

/* Create a wrapper for http.Client to add authorization token on
* every request.
 */
type httpClient struct {
	client *http.Client
	token  string
}

func (c *httpClient) Get(url string) (resp *http.Response, err error) {
	//Implement params & query params
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		//Return request forming error, for now panic
		panic("An error occured forming GET request")
	}
	return c.Do(req)
}

func (c *httpClient) Post(url string, body io.Reader) (resp *http.Response, err error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		//Return request forming error, for now panic
		panic("An error occured forming POST request")
	}
	return c.Do(req)
}

func (c *httpClient) Do(req *http.Request) (resp *http.Response, err error) {
	//Add headers
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.token))
	return c.client.Do(req)
}

func main() {
	router := gin.Default()
	router.Use(NumberPreformatter())
	router.POST("/c2b", c2b)
	router.Run("localhost:8000")
}

func NumberPreformatter() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var pushDetails push

		//Bind json to pusb
		if err := ctx.BindJSON(&pushDetails); err != nil {
			ctx.IndentedJSON(http.StatusInternalServerError, gin.H{
				"message": "A binding error occurred, required field: phone",
			})
		}

		//Validate phonenumber
		var invalidPhonenumber *InvalidPhonenumber
		phoneNumber, err := preformatNumber(pushDetails.Phone)
		if errors.As(err, &invalidPhonenumber) {
			ctx.IndentedJSON(http.StatusBadRequest, gin.H{
				"message": fmt.Sprintf("The phonenumber %s provided is not valid. %s", pushDetails.Phone, invalidPhonenumber.message),
			})
		}

		pushDetails.Phone = phoneNumber

		ctx.Set("details", pushDetails)
		ctx.Next()
	}
}

func c2b(ctx *gin.Context) {
	pushDetails := ctx.MustGet("details").(push)

	//C2B
	client := httpClient{
		client: &http.Client{},
		token:  "rEYc7jqeN4SNIuA4CTpkDMTDO6w9",
	}

	payload := &C2BRequest{
		ShortCode: businessShortCode,
		CommandID: commandId,
		Amount:    pushDetails.Amount,
		Msisdn:    pushDetails.Phone,
	}

	c2b, err := json.Marshal(payload)

	res, err := client.Post(url, bytes.NewBuffer(c2b))
	if err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, "Something went wrong while trying to create new c2b request")
		return
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		ctx.IndentedJSON(http.StatusInternalServerError, "Something went wrong while reading Mpesa C2B response")
		return
	}

	ctx.IndentedJSON(http.StatusAccepted, gin.H{"message": string(body)})
}

func preformatNumber(phone string) (string, error) {
	//Implement as a middleware
	phoneLen := len(phone)

	if phoneLen < 10 || phone == "" {
		//Return error invalid phonenumber
		return "", InvalidPhonenumber{message: fmt.Sprintf("%s is less than 10 digits or is empty", phone)}
	}

	if phoneLen == 10 && phone[:1] == "0" {
		//Remove the 0 and prepend the rest of the string with 254
		var builder strings.Builder
		builder.WriteString("254")
		builder.WriteString(phone[1:])
		phone = builder.String()
	}

	if phoneLen == 13 && phone[:1] == "+" {
		phone = phone[1:]
	}

	return phone, nil
}
