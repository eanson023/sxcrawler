package sxcrawler

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

//_URL 网址
const _URL = "http://jwgl.sanxiau.edu.cn"

var fileName string = "checkcode.gif"

// 发送请求的客户端
var client *http.Client = new(http.Client)

//RequestGeneral ...
type RequestGeneral struct {
	stu     *student
	cookies []*http.Cookie
	headers map[string]string
}

//loginForm 登录表单
type loginForm struct {
	// 不知道是啥
	__VIEWSTATE string
	// 学号
	TextBox1 string
	// 密码
	TextBox2 string
	// 验证码
	TextBox3 string
	//角色
	RadioButtonList1 string
	Button1          string
}

type student struct {
	xh       string
	password string
}

//成绩查询表单
type scoreForm struct {
	__VIEWSTATE string
	// 学年
	ddlXN string
	// 学期
	ddlXQ string
	// 安学期查询
	Button1 string
}

type studentInfo struct {
	// 学号
	stuID string
	// 姓名
	name string
	// 学院
	college string
	// 专业
	major string
	// 班级
	grade     string
	semesters []*semester
}

// 每学期信息
type semester struct {
	// 学年
	year string
	// 学期
	semester int
	// 所选学分
	selectedCredit float32
	// 所获学分
	gainCredit float32
	// 重修学分
	retakeCredit float32
	scores       *scores
}

type scores struct {

	// 课程名称
	courseCode string
	// 课程名称
	course string
	// 课程性质
	courseNature string
	// 课程归属

	// 学分
	credit string
	// 绩点
	gradePoint string
	// 成绩
	score string
	// 辅修标记
	minorMark string
	// 补考成绩
	retestScore string
	// 重修成绩
	retakeScore string
	// 学院名称
	collegeName string
	// 备注
	remarks string
	// 重修标记
	retakeMark string
}

func newStudent(xh string, pwd string) *student {
	return &student{
		xh:       xh,
		password: pwd,
	}
}

func newRequestGeneral() *RequestGeneral {
	return &RequestGeneral{
		cookies: []*http.Cookie{},
		headers: map[string]string{
			"User-Agent":      " Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/85.0.4183.83 Safari/537.36",
			"Accept-Language": " zh-CN,zh;q=0.9,en;q=0.8",
		},
	}
}

//addCookie 添加cookie进ReqHeader
func (rg *RequestGeneral) addCookie(cookies []*http.Cookie) {
	for _, cookie := range cookies {
		rg.cookies = append(rg.cookies, cookie)
	}
}

//addHeader 添加Header
func (rg *RequestGeneral) addHeader(key string, value string) {
	rg.headers[key] = value
}

func (rg *RequestGeneral) setReferer(value string) {
	rg.headers["Referer"] = value
}

func (rg *RequestGeneral) delReferer(value string) {
	rg.deleteHeader("Referer")
}

// deleteHeader 删除Header
func (rg *RequestGeneral) deleteHeader(key string) {
	delete(rg.headers, key)
}

// 将RequestHeader导入到http请求中
func (rg *RequestGeneral) headerIntoRequest(request *http.Request) {
	// 添加requestHeader
	for key, value := range rg.headers {
		request.Header.Add(key, value)
	}
}

//将cookie加到请求中
func (rg *RequestGeneral) cookieIntoRequest(request *http.Request) {
	// 添加cookie
	for _, cookie := range rg.cookies {
		request.AddCookie(cookie)
	}
}

// fmt.Printf("", var)
func (rg *RequestGeneral) newEmptyBodyRequest(method string, url string) *http.Request {
	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		panic(err)
	}
	// 将基本请求头加入到请求中
	rg.headerIntoRequest(request)
	// 将cookie加入请求中
	rg.cookieIntoRequest(request)
	return request
}

func (rg *RequestGeneral) newRequest(method string, url string, body io.Reader) *http.Request {
	request, err := http.NewRequest(method, url, body)
	if err != nil {
		panic(err)
	}
	// 将基本请求头加入到请求中
	rg.headerIntoRequest(request)
	// 将cookie加入请求中
	rg.cookieIntoRequest(request)
	return request
}

func (rg *RequestGeneral) newFormRequest(method string, url string, body io.Reader) *http.Request {
	req := rg.newRequest(method, url, body)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	return req
}

// 创建表单 根据字段名和其值进行映射
func (rg *RequestGeneral) generateForm(formPtr interface{}) *bytes.Buffer {
	getValue := reflect.ValueOf(formPtr).Elem()
	getType := reflect.TypeOf(formPtr).Elem()
	// x-www-form-urlencoded
	payload := &bytes.Buffer{}
	// 反射添加进条件中
	params := []string{}
	for i := 0; i < getValue.NumField(); i++ {
		str := getType.Field(i).Name + "=" + url.QueryEscape(getValue.Field(i).String())
		params = append(params, str)
	}
	ret := strings.Join(params, "&")
	payload.ReadFrom(strings.NewReader(ret))
	return payload
}

//newLoginForm 创建新的登录表单
func newLoginForm(username string, password string, checkcode string) *loginForm {
	return &loginForm{
		// 固定值
		__VIEWSTATE: "dDw3OTkxMjIwNTU7Oz6bmpbeSO1k01TBeZU9nxNbmYM4aw==",
		TextBox1:    username,
		TextBox2:    password,
		TextBox3:    checkcode,
		// 学生
		RadioButtonList1: "学生",
	}
}

//Login ... 程序入口
func Login(username string, password string) (*RequestGeneral, error) {
	fmt.Println("请注意不同IDE Console接收键盘输入问题O(∩_∩)O")
	rg := newRequestGeneral()
	cookies, err := getJwglCookies(rg)
	if err != nil {
		panic(err)
	}
	// 添加全局cookie
	rg.addCookie(cookies)
	// 睡眠
	time.Sleep(time.Millisecond * 500)
	// 输入验证码
	checkcode := inputCheckCode(rg)
	// 制造表单
	loginForm := newLoginForm(username, password, checkcode)
	// 再次睡眠
	time.Sleep(time.Millisecond * 500)
	// 登录
	err = login(rg, loginForm)
	if err != nil {
		return nil, err
	}
	// 放入同学信息
	rg.stu = newStudent(username, password)
	return rg, nil
}

// 获取教务系统cookie 里面包含sessionID
func getJwglCookies(reqGeneral *RequestGeneral) ([]*http.Cookie, error) {
	request := reqGeneral.newEmptyBodyRequest("GET", _URL)
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	return resp.Cookies(), nil
}

// 1. 获取cookie,通过cookie获取该验证码
func inputCheckCode(reqGeneral *RequestGeneral) string {
	var checkcode string
	checkCodeURL := "http://jwgl.sanxiau.edu.cn/CheckCode.aspx?"
	request := reqGeneral.newEmptyBodyRequest("GET", checkCodeURL)
	if resp, err := client.Do(request); err != nil {
		panic(err)
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		err = ioutil.WriteFile(fileName, body, os.ModePerm)
		if err != nil {
			fmt.Println("写入验证码文件出错")
			panic(err)
		}
		fmt.Println("请查看文件夹目录里的" + fileName + "文件")
		for checkcode == "" {
			fmt.Print("请输入验证码(回车结束):")
			fmt.Scanln(&checkcode)
		}
	}
	return checkcode
}

// 2. 模拟表单登录，随后根据Location跳转获取信息
// return 是否登录成功和响应结果
func login(reqGeneral *RequestGeneral, form *loginForm) error {
	loginURL := "http://jwgl.sanxiau.edu.cn/default2.aspx"
	// 添加
	param := reqGeneral.generateForm(form)
	request := reqGeneral.newFormRequest("POST", loginURL, param)
	resp, err := client.Do(request)
	if err != nil {
		panic(err)
	}
	reader, err := charset.NewReaderLabel("GBK", resp.Body)
	if err != nil {
		panic(err)
	}
	// 用goquery解析html 判断是否登录成功
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		panic(err)
	}
	// 如果登录失败了 网页上会有错误信息
	scriptText := doc.Find("script").Last().Text()
	wrong := strings.Contains(scriptText, "alert")
	if wrong {
		// 返回错误信息
		errorMsg := scriptText[strings.Index(scriptText, "(")+1 : strings.LastIndex(scriptText, ")")]
		return errors.New(errorMsg)
	}
	return nil
}

// GetScore 获取分数
func (rg *RequestGeneral) GetScore() error {
	mainPageURL := fmt.Sprintf("%v/xs_main.aspx?xh=%v", _URL, rg.stu.xh)
	req := rg.newEmptyBodyRequest("GET", mainPageURL)
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	doc, err := getUTF8DocumentFromReader(resp.Body)
	if err != nil {
		return err
	}
	// 获取第5栏 中的子ul中的li的链接
	uri, _ := getNavLi(doc, 4).Find("li").Last().Children().Attr("href")
	xscjURL := fmt.Sprintf("%v/%v", _URL, uri)
	rg.setReferer(xscjURL)
	req = rg.newEmptyBodyRequest("GET", xscjURL)
	resp, err = client.Do(req)
	if err != nil {
		// 网站设置防爬手段不加Referer报 302 response missing Location header
		panic(err)
	}
	// 学年
	years := []string{}
	//学期
	smsters := []int{}
	doc, err = getUTF8DocumentFromReader(resp.Body)
	if err != nil {
		return err
	}
	// 基本信息栏
	searchCon := doc.Find("p[class=search_con]")
	fmt.Println(searchCon.Html())
	studentInfo := new(studentInfo)
	// 获取基本信息
	{
		fmt.Println("***********************")
		fmt.Println(searchCon.Find("#Label3").Html())
		studentInfo.stuID = strings.Split(searchCon.Find("#Label3").Text(), "：")[1]
		studentInfo.name = strings.Split(searchCon.Find("#Label5").Text(), "：")[1]
		studentInfo.college = strings.Split(searchCon.Find("#Label6").Text(), "：")[1]
		studentInfo.major = searchCon.Find("#Label7").Text()
		studentInfo.grade = strings.Split(searchCon.Find("#Label8").Text(), "：")[1]
		// 学年
		searchCon.Find("select[name=ddlXN]").Children().Each(func(i int, s *goquery.Selection) {
			if s.Text() == "" {
				return
			}
			years = append(years, s.Text())
		})
		// 学期
		searchCon.Find("select[name=ddlXQ]").Children().Each(func(i int, s *goquery.Selection) {
			if s.Text() == "" {
				return
			}
			v, _ := strconv.Atoi(s.Text())
			smsters = append(smsters, v)
		})
	}
	// 获取表单中viewstate值
	viewstate, _ := doc.Find("input[name=__VIEWSTATE]").Attr("value")
	// 获取表单value值
	button1, _ := searchCon.Find("#Button1").Attr("value")
	// 并发获取每学期信息
	{
		var wg sync.WaitGroup
		// var idx int32 = 0
		// semesters := []semester{}
		for i := 0; i < len(years); i++ {
			for j := 0; j < len(smsters); j++ {
				wg.Add(1)
				go func(yearIdx int, xqIdx int) {
					form := new(scoreForm)
					form.__VIEWSTATE = viewstate
					form.Button1 = button1
					form.ddlXN = years[yearIdx]
					form.ddlXQ = strconv.Itoa(smsters[xqIdx])
					reader := rg.generateForm(form)
					request := rg.newFormRequest("POST", xscjURL, reader)
					// 为避免并发锁争抢问题 每个协程使用自己的client
					cli := &http.Client{}
					response, err := cli.Do(request)
					if err != nil {
						panic(err)
					}
					PrintResponse(response, GBK)
					fmt.Println("*************************************")
					wg.Done()
				}(i, j)
			}
		}
		wg.Wait()
	}
	return nil
}

// 将GBK编码的reader转为utf8格式的reader
func getUTF8DocumentFromReader(body io.ReadCloser) (*goquery.Document, error) {
	reader, err := charset.NewReaderLabel("GBK", body)
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// getNavLi 根据index获取第几个导航栏的信息
func getNavLi(doc *goquery.Document, index int) *goquery.Document {
	// 从导航栏获取信息查询栏
	selection := doc.Find("#headDiv>.nav>.top")
	// 获取倒数第2栏 中的子ul中的li的链接
	return goquery.NewDocumentFromNode(selection.Get(index))
}

//Done 结束删除验证码文件
func Done() {
	os.Remove(fileName)
}
