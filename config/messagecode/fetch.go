package messagecode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"code.finan.one/finan-one-be/fo-utils/net/uthttp"
	"code.finan.one/finan-one-be/fo-utils/utils/utfunc"
	redis "github.com/redis/go-redis/v9"
	"gitlab.com/goxp/cloud0/logger"
)

const (
	generalGroup = 10

	messageGroupEmptyKey = "empty"
)

type messageCode struct {
	HTTPCode int    `json:"http_code"`
	Message  string `json:"messasge"`
}

type strapiMessageCodeResp struct {
	Data  []strapiMessageCode `json:"data"`
	Error struct {
		Status  int                    `json:"status"`
		Name    string                 `json:"name"`
		Message string                 `json:"message"`
		Details ValidationErrorDetails `json:"details"`
	} `json:"error"`
	Meta strapiMeta `json:"meta"`
}

type strapiMessageCode struct {
	ID       int    `json:"id"`
	Code     int    `json:"code"`
	Locale   string `json:"locale"`
	Message  string `json:"message"`
	HTTPCode int    `json:"http_code"`
}

type strapiMeta struct {
	Pagination strapiPagination `json:"pagination"`
}

type strapiPagination struct {
	Page      int `json:"page"`
	PageSize  int `json:"pageSize"`
	PageCount int `json:"pageCount"`
	Total     int `json:"total"`
}

type Config struct {
	RedisAddr            string
	RedisPwd             string
	RedisDB              int
	StrapiMessageCodeURL string
	StrapiToken          string
	MessageGroup         []int
}

type Client struct {
	redisCli   *redis.Client
	cfg        Config
	messageMap map[string]messageCode
}

func NewClient(cfg Config) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPwd,
		DB:       cfg.RedisDB,
	})

	client := &Client{
		redisCli:   rdb,
		cfg:        cfg,
		messageMap: map[string]messageCode{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := client.LoadMessageCode(ctx, cfg.MessageGroup...)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (c *Client) GetMessage(locale string, code int) string {
	messCode, ok := c.messageMap[makeFieldKey(locale, code)]
	if !ok {
		return getDefaultLocaleMessage(locale)
	}

	return messCode.Message
}

func (c *Client) GetHTTPCode(locale string, code int) int {
	messCode, ok := c.messageMap[makeFieldKey(locale, code)]
	if !ok {
		return fallbackMessageCodeToHTTPCode(code)
	}

	return messCode.HTTPCode
}

// Load messages from redis if cache hit or from strapi if cache miss. Ignore all error.
func (c *Client) LoadMessageCode(ctx context.Context, messageGroups ...int) error {
	log := logger.WithCtx(ctx, utfunc.GetCurrentCaller(c, 0))

	messageGroups = append(messageGroups, generalGroup)
	for _, group := range messageGroups {
		key := makeHashKey(group)
		//messageGroupRes, err := c.redisCli.HGetAll(ctx, key).Result()
		//var cacheHit bool
		//if err != nil {
		//	log.WithError(err).Errorf("Failed to get message codes of group %d from redis", group)
		//	cacheHit = false
		//	err = nil
		//} else if len(messageGroupRes) == 0 {
		//	cacheHit = false
		//} else {
		//	cacheHit = true
		//}
		//if cacheHit {
		//	messageStrMap := messageGroupRes
		//	_, ok := messageStrMap[messageGroupEmptyKey]
		//	if ok {
		//		continue
		//	}
		//	messageCodeMap, err := byteMapToMessageCodeMap(messageStrMap)
		//	if err != nil {
		//		return err
		//	}
		//
		//	c.mergeMessageCodesMap(messageCodeMap)
		//} else {
		messageCodeMap, err := c.getMessageGroupMapFromStrapi(ctx, group)
		if err != nil {
			log.WithError(err).Errorf("Failed to get message codes of group %d from strapi", group)
			continue
		}

		if len(messageCodeMap) == 0 {
			messageCodeMap[messageGroupEmptyKey] = messageCode{}
		}

		anyMap, err := messageMapToAnyMap(messageCodeMap)
		if err != nil {
			return err
		}

		_ = c.redisCli.HMSet(ctx, key, anyMap)

		c.mergeMessageCodesMap(messageCodeMap)
		//}
	}

	return nil
}

func (c *Client) getMessageGroupMapFromStrapi(ctx context.Context, messageGroup int) (map[string]messageCode, error) {
	res := map[string]messageCode{}
	messageCodes, err := c.getStrapiMessageCodes(ctx, messageGroup)
	if err != nil {
		return nil, err
	}
	for _, messCode := range messageCodes {
		res[makeFieldKey(messCode.Locale, messCode.Code)] = messageCode{
			HTTPCode: messCode.HTTPCode,
			Message:  messCode.Message,
		}
	}

	return res, nil
}

func messageMapToAnyMap(messageMap map[string]messageCode) (map[string]any, error) {
	byteMap := make(map[string]any, len(messageMap))

	for key, val := range messageMap {
		blob, err := json.Marshal(val)
		if err != nil {
			return nil, err
		}

		byteMap[key] = blob
	}

	return byteMap, nil
}

func byteMapToMessageCodeMap(byteMap map[string]string) (map[string]messageCode, error) {
	messsageCodeMap := make(map[string]messageCode, len(byteMap))
	for key, val := range byteMap {
		var messCode messageCode
		err := json.Unmarshal([]byte(val), &messCode)
		if err != nil {
			return nil, err
		}

		messsageCodeMap[key] = messCode
	}

	return messsageCodeMap, nil
}

func makeHashKey(messageGroup int) string {
	return fmt.Sprintf("messagegroup:%d", messageGroup)
}

func makeFieldKey(locale string, messageCode int) string {
	return fmt.Sprintf("%s:%d", locale, messageCode)
}

func fallbackMessageCodeToHTTPCode(code int) int {
	messCodeStr := fmt.Sprintf("%d", code)

	if len(messCodeStr) != 6 {
		return http.StatusInternalServerError
	}

	switch messCodeStr[2] {
	case '2':
		return http.StatusOK
	case '4':
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func (c *Client) getStrapiMessageCodes(ctx context.Context, messageGroup int) ([]strapiMessageCode, error) {
	totalPage := 1
	page := 1
	uri, err := url.Parse(c.cfg.StrapiMessageCodeURL)
	if err != nil {
		return nil, err
	}
	var messageCodes []strapiMessageCode
	for page <= totalPage {
		queryVals := uri.Query()
		queryVals.Set("pagination[page]", fmt.Sprintf("%d", page))
		queryVals.Set("pagination[pageSize]", "300")
		queryVals.Set("locale", "all")
		queryVals.Set("filters[code][$startsWithi]", strconv.Itoa(messageGroup))
		uri.RawQuery = queryVals.Encode()

		req := uthttp.HTTPRequest{
			Method: http.MethodGet,
			URL:    uri.String(),
			Silent: true,
			Header: map[string]string{
				"Authorization": fmt.Sprintf("Bearer %s", c.cfg.StrapiToken),
			},
		}
		cfg := uthttp.Config{
			Timeout: 3 * time.Second,
		}

		client := uthttp.NewHTTPClient(cfg)

		resp, err := uthttp.SendHTTPRequest[strapiMessageCodeResp](ctx, client, req, uthttp.DefaultOptions())
		if err != nil {
			return nil, err
		}

		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("get message codes from strapi status code: %d", resp.StatusCode)
		}

		body := resp.Body

		messageCodes = append(messageCodes, body.Data...)

		totalPage = body.Meta.Pagination.PageCount
		page++
	}

	return messageCodes, nil
}

func (c *Client) mergeMessageCodesMap(messageMap map[string]messageCode) {
	for key, val := range messageMap {
		c.messageMap[key] = val
	}
}

func (c *Client) PublishMessageCode(ctx context.Context, req CreateMessageCodeReq) (interface{}, error) {

	var unifiedResponse interface{}
	if req.EnMessage == "" || req.ViMessage == "" {
		return nil, errors.New("missing English message or Vietnamese message")
	}

	// Prepare the initial POST request
	req.Locale = "en"
	req.Message = req.EnMessage
	httpReq := uthttp.HTTPRequest{
		Method: http.MethodPost,
		URL:    c.cfg.StrapiMessageCodeURL,
		Header: map[string]string{
			"Authorization": fmt.Sprintf("Bearer %s", c.cfg.StrapiToken),
		},
		Body:   req,
		LogTag: "PublishMessageCode",
	}

	client := uthttp.NewHTTPClient(uthttp.Config{
		Timeout: 3 * time.Second,
	})

	// Send the initial POST request
	res, err := uthttp.SendHTTPRequest[json.RawMessage](ctx, client, httpReq, uthttp.DefaultOptions())
	if err != nil {
		return unifiedResponse, err
	}

	if res.StatusCode >= 400 {
		var errorResponse ErrorResponse
		if err := json.Unmarshal(res.Body, &errorResponse); err != nil {
			return unifiedResponse, err
		}
		unifiedResponse = &errorResponse
	} else {
		var successResponse SuccessResponse
		if err := json.Unmarshal(res.Body, &successResponse); err != nil {
			return unifiedResponse, err
		}
		unifiedResponse = &successResponse

		// Prepare the PUT request to update the English message
		req.Locale = "vi"
		req.Message = req.ViMessage
		req.Localizations = []int{successResponse.Data.ID}
		httpReq = uthttp.HTTPRequest{
			Method: "POST",
			URL:    c.cfg.StrapiMessageCodeURL,
			Header: map[string]string{
				"Authorization": fmt.Sprintf("Bearer %s", c.cfg.StrapiToken),
			},
			Body:   req,
			LogTag: "PublishMessageCode",
		}

		res, err = uthttp.SendHTTPRequest[json.RawMessage](ctx, client, httpReq, uthttp.DefaultOptions())
		if err != nil {
			return unifiedResponse, err
		}

		if res.StatusCode >= 400 {
			var errorResponse ErrorResponse
			if err := json.Unmarshal(res.Body, &errorResponse); err != nil {
				return unifiedResponse, err
			}
			unifiedResponse = &errorResponse
		} else {
			var updateResponse SuccessResponse
			if err := json.Unmarshal(res.Body, &updateResponse); err != nil {
				return unifiedResponse, err
			}
			unifiedResponse = &updateResponse
		}
	}

	return unifiedResponse, nil
}
