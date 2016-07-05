package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/axgle/mahonia"
)

const (
	userAgent       = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_3) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/48.0.2564.116 Safari/537.36"
	ltRegStr        = `name="lt" value="(.+?)"`
	executionRegStr = `name="execution" value="(.+?)"`
)

var (
	ErrNotLogined     = errors.New("not logined")
	ErrBodyNotMatched = errors.New("body not matched")
	ltReg             = regexp.MustCompile(ltRegStr)
	executionReg      = regexp.MustCompile(executionRegStr)
)

type MyJar struct {
	http.CookieJar
	cookies map[string][]*http.Cookie
}

func (j *MyJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.cookies[u.Host] = cookies
}

func (j *MyJar) Cookies(u *url.URL) []*http.Cookie {
	return j.cookies[u.Host]
}

func NewMyJar() *MyJar {
	return &MyJar{
		cookies: make(map[string][]*http.Cookie),
	}
}

type SogouAdService interface {
	FetchLoginParams() (lt, execution string, err error)
	Login(lt, execution, username, password, captcha string) error
	KeepLogined() error
	CheckLogined() error
	FetchCaptcha() ([]byte, error)
	DownloadReport(param *QueryReportParam) ([]byte, error)
}

type sogouAdService struct {
	jar    *MyJar
	client *http.Client
}

func NewSogouAdService() SogouAdService {
	jar := NewMyJar()
	return &sogouAdService{
		jar: jar,
		client: &http.Client{
			Jar:     jar,
			Timeout: time.Second * 10,
		},
	}
}

func (s *sogouAdService) ts() int64 {
	return time.Now().UnixNano() / 1e6
}
func (s *sogouAdService) FetchLoginParams() (lt, execution string, err error) {
	u := fmt.Sprintf("https://auth.p4p.sogou.com/login?service=http%%3A%%2F%%2Fxuri.p4p.sogou.com%%2Fcpcadindex%%2Finit.action&nonce=%d", s.ts())

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Referer", "http://xuri.p4p.sogou.com/cpcadindex/init.action")

	res, err := s.client.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}

	matches := ltReg.FindSubmatch(body)
	if matches == nil {
		err = ErrBodyNotMatched
		return
	}
	lt = string(matches[1])

	matches = executionReg.FindSubmatch(body)
	if matches == nil {
		err = ErrBodyNotMatched
		return
	}
	execution = string(matches[1])

	return
}

func (s *sogouAdService) Login(lt, execution, username, password, captcha string) (err error) {
	u := "https://auth.p4p.sogou.com/login?service=http://xuri.p4p.sogou.com"

	body := strings.NewReader(url.Values{
		"lt":           {lt},
		"execution":    {execution},
		"_eventId":     {"submit"},
		"username":     {username},
		"password":     {password},
		"validateCode": {captcha},
	}.Encode())

	req, err := http.NewRequest("POST", u, body)
	if err != nil {
		return
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Origin", "https://auth.p4p.sogou.com")
	req.Header.Set("Referer", "https://auth.p4p.sogou.com/login?service=http%3A%2F%2Fxuri.p4p.sogou.com%2F%2Fjsp%2Fwelcome.jsp&nonce=1456755619438")
	res, err := s.client.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()

	return
}

func (s *sogouAdService) FetchCaptcha() ([]byte, error) {
	ts := s.ts()
	u := fmt.Sprintf("https://auth.p4p.sogou.com/validateCode/%d?code=checkcode&nonce=%d", ts, ts)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)
	res, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return ioutil.ReadAll(res.Body)
}

// {"timeSelect":"5","startDate":"2016-02-15","endDate":"2016-02-21","statType":"0","reportType":"3","deviceType":"0"}
type QueryReportParam struct {
	TimeSelect string `json:"timeSelect"`
	StartDate  string `json:"startDate"`
	EndDate    string `json:"endDate"`
	StatType   string `json:"statType"`
	ReportType string `json:"reportType"`
	DeviceType string `json:"deviceType"`
}

func (s *sogouAdService) DownloadReport(param *QueryReportParam) (content []byte, err error) {
	u := "http://xuri.p4p.sogou.com/report/common/downloadReport.action"
	jsonStr, err := json.Marshal(param)
	if err != nil {
		return
	}

	body := bytes.NewBufferString("jsonStr=")
	body.Write(jsonStr)

	req, err := http.NewRequest("POST", u, body)
	if err != nil {
		return
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "http://xuri.p4p.sogou.com/cpcadindex/init.action")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Origin", "http://xuri.p4p.sogou.com")

	res, err := s.client.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()

	r := mahonia.NewDecoder("gbk").NewReader(res.Body)
	return ioutil.ReadAll(r)
}

func (s *sogouAdService) CheckLogined() (err error) {
	u := "http://xuri.p4p.sogou.com/cpcadindex/init.action"

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	client := &http.Client{
		Jar:     s.jar,
		Timeout: time.Second * 10,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return ErrNotLogined
		},
	}
	res, err := client.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()

	return
}

func (s *sogouAdService) KeepLogined() (err error) {
	u := fmt.Sprintf("http://xuri.p4p.sogou.com/report/account/overview.action?t=%d", s.ts())

	req, err := http.NewRequest("POST", u, nil)
	if err != nil {
		return
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("Referer", "http://xuri.p4p.sogou.com/cpcadindex/init.action")
	req.Header.Set("Sogou-Hash", "#datareport/account/list")
	req.Header.Set("Sogou-Request-Type", "XMLHTTPRequest")
	req.Header.Set("Origin", "http://xuri.p4p.sogou.com")

	res, err := s.client.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()

	return
}
