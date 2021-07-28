package main

import (
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"fmt"
	"time"

	"github.com/gogf/gf/encoding/gjson"
	"github.com/gogf/gf/frame/g"
	"github.com/gogf/gf/net/ghttp"
	qrcode "github.com/skip2/go-qrcode"
)

var QLheader map[string]string
var path string
var QLurl string
var Config string = `
#公告设置
[app]
    path            = "QL" #青龙面板映射文件夹名称,一般为QL或ql
    QLip            = "http://127.0.0.1" #青龙面板的ip
    QLport          = "5700" #青龙面板的端口，默认为5700
    notice          = "使用京东扫描二维码登录" #公告/说明
    pushQr          = "" #消息推送二维码链接
    logName         = "chinnkarahoi_jd_scripts_jd_bean_change" #日志脚本名称
    allowAdd        = 0 #是否允许添加账号（0允许1不允许）不允许添加时则只允许已有账号登录
    allowNum        = 99 #允许添加账号的最大数量,-1为不限制


#web服务设置
[server]
    address         = ":5701" #端口号设置
    serverRoot      = "public" #静态目录设置，请勿更改
    serverAgent     = "JDCookie" #服务端UA

#模板设置
[viewer]
    Delimiters      = ["${", "}"] #模板标签，请勿更改
`

func main() {
	//检查配置文件
	checkConfig()

	//设置ptah
	path = g.Cfg().GetString("app.path")

	//设置接口
	QLurl = g.Cfg().GetString("app.QLip") + ":" + g.Cfg().GetString("app.QLport")

	//获取auth
	getAuth()

	//WEB服务
	s := g.Server()

	//允许跨域
	s.BindMiddlewareDefault(func(r *ghttp.Request) {
		getAuth()
		r.Response.CORSDefault()
		r.Middleware.Next()
	})

	s.BindHandler("/info", func(r *ghttp.Request) {
		r.Response.WriteExit("JDC is already!")
	})
	s.BindHandler("/qrcode", func(r *ghttp.Request) {
		result := getQrcode()
		r.Response.WriteJsonExit(result)
	})
	s.BindHandler("/check", func(r *ghttp.Request) {
		token := r.GetString("token")
		okl_token := r.GetString("okl_token")
		cookies := r.GetString("cookies")
		code, data := checkLogin(token, okl_token, cookies)
		if code != 0 {
			r.Response.WriteJsonExit(g.Map{"code": code, "data": data})
		} else {
			code, res := addCookie(data)
			//获取cid
			_, cid := getId(data)
			r.Response.WriteJsonExit(g.Map{"code": code, "data": res, "cid": cid})
		}

	})
	s.BindHandler("/delete", func(r *ghttp.Request) {
		cid := r.GetString("cid")
		cookieDel(cid)
		r.Response.WriteJsonExit(g.Map{"code": 0, "data": "已成功从系统中移除你的账号！"})

	})
	s.BindHandler("/notice", func(r *ghttp.Request) {
		r.Response.WriteJsonExit(g.Map{"code": 0, "data": g.Cfg().GetString("app.notice")})
	})
	s.BindHandler("/push_qr", func(r *ghttp.Request) {
		r.Response.WriteJsonExit(g.Map{"code": 0, "data": g.Cfg().GetString("app.pushQr")})
	})
	s.BindHandler("/checkcookie", func(r *ghttp.Request) {
		cid := r.GetString("cid")
		res := checkCookie(cid)
		r.Response.WriteExit(res)
	})
	s.BindHandler("/log", func(r *ghttp.Request) {
		cid := r.GetString("cid")
		logs := getUserLog(cid)
		r.Response.WriteJsonExit(g.Map{"code": 0, "data": logs})

	})
	s.BindHandler("/node_info", func(r *ghttp.Request) {
		res := nodeInfo()
		r.Response.WriteJsonExit(res)

	})
	s.Run()
}

//获取服务器信息
func nodeInfo() interface{} {
	cookies := getCookieList2()
	allow := g.Cfg().GetInt("app.allowNum")
	now := len(cookies)
	var isAllow bool
	var Num int
	if allow > now {
		Num = allow - now + 1
		isAllow = true
	} else if allow == -1 {
		Num = -1
		isAllow = true
	} else {
		Num = 0
		isAllow = false
	}

	//检查是否允许添加
	allowAdd := g.Cfg().GetInt("app.allowAdd")
	if allowAdd != 0 {
		Num = 0
		isAllow = false
	}
	return g.Map{"code": 0, "isAllow": isAllow, "Num": Num}
}

//截取目标段落
func getUserLog(ccid string) string {
	var wz int = 0
	var flag bool = false
	var all int = 0
	//判断用户账号位置

	ckList := cookieList()
	if ckList == `{"code":200,"data":[]}` {
		return "error"
	}
	if j, err := gjson.DecodeToJson(ckList); err != nil {
		log.Println("error！can't read the auth file!")
	} else {
		data := j.GetArray("data")
		//检查账号
		var i = 0
		for _, v := range data {
			i++
			val, ok := v.(g.Map)
			if !ok {
				log.Println("no")
			}
			//获取id
			id := val["_id"]
			cid, ok := id.(string)
			if !ok {
				log.Println("noid")
			}
			//判断如果一致，返回
			if cid == ccid {
				flag = true
				wz = i
			}

		}
		all = i
		if !flag {
			return "未找到该用户！"
		}

	}
	//截取目标段落
	logRaw := getLog()
	var re *regexp.Regexp
	if wz == all {
		re = regexp.MustCompile(`(\*\*\*\*\*\*\*\*开始【京东账号` + strconv.Itoa(wz) + `】[\s\S]*🧧\n)`)
	} else {
		re = regexp.MustCompile(`(\*\*\*\*\*\*\*\*开始【京东账号` + strconv.Itoa(wz) + `】[\s\S]*?)\*\*\*\*\*\*\*\*开始【京东账号`)
	}
	reJ := re.FindStringSubmatch(logRaw)
	if reJ == nil {
		return "暂无日志！请明天再来查看！"
	}

	re2 := regexp.MustCompile(`==================脚本执行.*?=========`)
	re2J := re2.FindStringSubmatch(logRaw)
	return re2J[0] + "\n" + reJ[1]

}

//获取日志文件
func getLog() string {
	var fileName string
	var result string
	var logName string
	loc, _ := time.LoadLocation("Asia/Shanghai")
	Ntime := strconv.FormatInt(time.Now().In(loc).Unix(), 10)
	c := g.Client()
	c.SetHeaderMap(QLheader)

	r, err := c.Get(QLurl + "/api/logs?t=" + Ntime)
	if err != nil {
		log.Println("error!Please check QLip and QLport!errCode:1002")
		os.Exit(1)
	}
	defer r.Close()
	if j, err := gjson.DecodeToJson(r.ReadAllString()); err != nil {
		log.Println("error！can't read the auth file!")
	} else {
		dirs := j.GetArray("dirs")
		//循环获取dirs数组
		for _, v := range dirs {
			val, ok := v.(g.Map)
			if !ok {
				log.Println("noval")
			}
			namev := val["name"]
			name, ok := namev.(string)
			if !ok {
				log.Println("noval")
			}
			logName = g.Cfg().GetString("app.logName")
			if logName == "" {
				logName = "chinnkarahoi_jd_scripts_jd_bean_change"
			}
			if name == logName {
				filesv := val["files"]
				files, ok := filesv.(g.Array)
				if !ok {
					log.Println("nofiles")
				}
				fileName, ok = files[0].(string)
				if !ok {
					log.Println("nofileName")
				}
			}

		}
	}
	//获取文件内容
	res, _ := c.Get(QLurl + "/api/logs/" + logName + "/" + fileName + "?t=" + Ntime)
	defer res.Close()
	if j, err := gjson.DecodeToJson(res.ReadAllString()); err != nil {
		log.Println("error！can't read the auth file!")
	} else {
		result = j.GetString("data")
	}
	return result

}

//账号状态检测
func checkCookie(ccid string) string {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	Ntime := strconv.FormatInt(time.Now().In(loc).Unix(), 10)
	c := g.Client()
	c.SetHeaderMap(QLheader)

	r, _ := c.Get(QLurl + "/api/cookies/" + ccid + "/refresh?t=" + Ntime)
	defer r.Close()
	return r.ReadAllString()
}

//获取QLID
func getId(cookie string) (int, string) {
	var result string
	var isTrue bool = false
	//获取cookie中的pt_pin
	re2 := regexp.MustCompile("pt_pin=(.*?);")
	re2J := re2.FindStringSubmatch(cookie)
	pin2 := re2J[1]

	//获取cookie列表
	ckList := getCookieList()
	ckList2 := getCookieList2()
	if ckList == nil || ckList2 == nil {
		return 1, "该账号不存在！"
	}

	var oldCk string
	//获取原cookie
	for i := 0; i < len(ckList2); i++ {
		if ckList2[i] == "" {
			continue
		}
		//获取cookie中的pt_pin
		re3J := re2.FindStringSubmatch(ckList2[i])
		pin3 := re3J[1]
		if pin3 == pin2 {
			oldCk = ckList2[i]
		}
	}

	if oldCk == "" {
		return 1, "未知错误！"
	}

	//检查账号
	for _, v := range ckList {
		j, err := gjson.DecodeToJson(v)
		if err != nil {
			log.Println("error！can't read cookieList!")
			continue
		}

		//获取cookie
		ck := j.GetString("value")

		//获取cid
		cid := j.GetString("_id")

		//判断如果一致，返回cid
		if oldCk == ck {
			isTrue = true
			result = cid
			break
		}

	}

	if isTrue {
		return 0, result
	} else {
		return 1, "不存在！"
	}

}

//删除cookie
func cookieDel(id string) string {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	Ntime := strconv.FormatInt(time.Now().In(loc).Unix(), 10)
	c := g.Client()
	c.SetHeaderMap(QLheader)

	r, _ := c.Delete(QLurl+"/api/cookies?t="+Ntime, `["`+id+`"]`)
	defer r.Close()
	return r.ReadAllString()
}

//新增cookie
func cookieAdd(value string) string {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	Ntime := strconv.FormatInt(time.Now().In(loc).Unix(), 10)
	c := g.Client()
	c.SetHeaderMap(QLheader)

	r, _ := c.Post(QLurl+"/api/cookies?t="+Ntime, `["`+value+`"]`)
	defer r.Close()

	return r.ReadAllString()
}

//更新cookie
func cookieUpdate(id string, value string) string {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	Ntime := strconv.FormatInt(time.Now().In(loc).Unix(), 10)
	c := g.Client()
	c.SetHeaderMap(QLheader)

	r, _ := c.Put(QLurl+"/api/cookies?t="+Ntime, `{"_id":"`+id+`","value":"`+value+`"}`)
	defer r.Close()

	return r.ReadAllString()
}

//获取cookie列表
func cookieList() string {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	Ntime := strconv.FormatInt(time.Now().In(loc).Unix(), 10)
	c := g.Client()
	c.SetHeaderMap(QLheader)

	r, err := c.Get(QLurl + "/api/cookies?t=" + Ntime)
	if err != nil {
		log.Println("error!Please check QLip and QLport!errCode:1001")
		os.Exit(1)
	}
	defer r.Close()

	return r.ReadAllString()
}

//检查配置文件
func checkConfig() {
	_, err := os.Stat("config.toml")
	if err == nil {
		log.Println("Success to loading config!")
	}

	if os.IsNotExist(err) {
		f, err := os.Create("config.toml")
		if err != nil {
			log.Println(err.Error())
		} else {
			log.Println("The config file was generated successfully！Please restart this program")
			f.Write([]byte(Config))
			os.Exit(0)
		}
		defer f.Close()
	}
	//检查public
	_, err = os.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return
		}
		if os.IsNotExist(err) {
			os.MkdirAll("./public", os.ModePerm)
			return
		}
		return
	}
}

//获取auth
func getAuth() {
	//读取文件
	f, err := os.OpenFile(path+"/config/auth.json", os.O_RDONLY, 0766)
	if err != nil {
		log.Println(err.Error())
	}
	defer f.Close()
	con, _ := ioutil.ReadAll(f)
	//解析结果
	if j, err := gjson.DecodeToJson(string(con)); err != nil {
		log.Println("error！can't read the auth file!")
		os.Exit(1)
	} else {
		QLheader = map[string]string{"Authorization": "Bearer " + j.GetString("token")}
	}
}

//获取cookie列表
func getCookieList() []string {
	//读取文件
	f, err := os.OpenFile(path+"/db/cookie.db", os.O_RDONLY, 0766)
	if err != nil {
		log.Println(err.Error())
	}
	defer f.Close()
	con, _ := ioutil.ReadAll(f)
	//解析结果
	list := strings.Split(string(con), "\n")
	return list
}

//获取cookie列表
func getCookieList2() []string {
	//读取文件
	f, err := os.OpenFile(path+"/config/cookie.sh", os.O_RDONLY, 0766)
	if err != nil {
		log.Println(err.Error())
	}
	defer f.Close()
	con, _ := ioutil.ReadAll(f)
	//解析结果
	list := strings.Split(string(con), "\n")
	return list
}

//登录添加cookie
func addCookie(cookie string) (int, string) {
	var isNew bool = true
	//获取cookie中的pt_pin
	re2 := regexp.MustCompile("pt_pin=(.*?);")
	re2J := re2.FindStringSubmatch(cookie)
	pin2 := re2J[1]

	//获取cookie列表
	ckList := getCookieList()
	ckList2 := getCookieList2()
	if ckList == nil {
		//检查是否允许添加
		allowAdd := g.Cfg().GetInt("app.allowAdd")
		if allowAdd != 0 {
			return 400, "该节点不允许添加账号！"
		}
		//检查是否超过账号限制
		allowNum := g.Cfg().GetInt("app.allowNum")
		nowNum := len(ckList2)
		if allowNum <= nowNum && allowNum != -1 {
			return 400, "该节点账号已达上限，请更换节点添加！"
		}
		cookieAdd(cookie)
		return 0, "添加成功！"
	}

	//检查是否是新增账号
	//是否存在
	for _, v := range ckList2 {
		if v == "" {
			continue
		}
		//获取cookie中的pt_pin
		re := regexp.MustCompile("pt_pin=(.*?);")
		reJ := re.FindStringSubmatch(v)
		pin1 := reJ[1]
		//判断如果一致，更新账号
		if pin1 == pin2 {
			isNew = false
		}

	}
	if !isNew {
		var dbCid string = ""
		for _, v := range ckList {
			if v == "" {
				continue
			}
			j, err := gjson.DecodeToJson(v)
			//解析结果
			if err != nil {
				log.Println("error！Json read error!")
				continue
			}
			//获取cookie
			cookieT := j.GetString("value")

			//获取id
			cid := j.GetString("_id")

			log.Println(v)
			//获取cookie中的pt_pin
			re := regexp.MustCompile("pt_pin=(.*?);")
			reJ := re.FindStringSubmatch(cookieT)
			if len(reJ) < 2 {
				continue
			}
			pin1 := reJ[1]
			//判断如果一致，更新账号
			if pin1 == pin2 {
				dbCid = cid
			}

		}
		cookieUpdate(dbCid, cookie)
		return 0, "更新成功"
	}

	if isNew {
		//检查是否允许添加
		allowAdd := g.Cfg().GetInt("app.allowAdd")
		if allowAdd != 0 {
			return 400, "该节点不允许添加账号！"
		}
		//检查是否超过账号限制
		allowNum := g.Cfg().GetInt("app.allowNum")
		nowNum := len(ckList2)
		if allowNum <= nowNum && allowNum != -1 {
			return 400, "账号已达上限，请更换节点添加！"
		}
		cookieAdd(cookie)
		return 0, "添加成功"
	} else {
		return 0, "更新成功"
	}

}

//解析cookie
func parseCookie(raw string) map[string]string {
	result := make(map[string]string)
	re := regexp.MustCompile(`Set-Cookie:(.*?;)`)
	matched := re.FindAllStringSubmatch(raw, -1)
	for _, v := range matched {
		tmp := strings.ReplaceAll(v[1], " ", "")
		re2 := regexp.MustCompile("(.*?)=(.*?);")
		re2J := re2.FindStringSubmatch(tmp)
		k := re2J[1]
		pas := re2J[2]
		if pas == "" {
			continue
		}
		result[k] = pas

	}
	return result

}

//获取随机UA
var UA = func() string {
	return "jdapp;android;12.0.5;11;0393465333165363-" + fmt.Sprint(time.Now().Unix()) + ";network/wifi;model/M2102K1C;osVer/30;appBuild/88681;partner/lc001;eufv/1;jdSupportDarkMode/0;Mozilla/5.0 (Linux; Android 11; M2102K1C Build/RKQ1.201112.002; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/77.0.3865.120 MQQBrowser/6.2 TBS/045534 Mobile Safari/537.36"
}

//检测登录
func checkLogin(token string, okl_token string, cookies string) (int, string) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	Ntime := strconv.FormatInt(time.Now().In(loc).Unix(), 10)
	getUserCookieUrl := `https://plogin.m.jd.com/cgi-bin/m/tmauthchecktoken?&token=` + token + `&ou_state=0&okl_token=` + okl_token
	loginUrl := "https://plogin.m.jd.com/cgi-bin/mm/new_login_entrance?lang=chs&appid=300&returnurl=https://wq.jd.com/passport/LoginRedirect?state=" + Ntime + "&returnurl=https://home.m.jd.com/myJd/newhome.action?sceneval=2&ufc=&/myJd/home.action&source=wq_passport"
	headers := map[string]string{
		"Connection":      "Keep-Alive",
		"Content-Type":    "application/x-www-form-urlencoded",
		"Accept":          "application/json, text/plain, */*",
		"Accept-Language": "zh-cn",
		"Cookie":          cookies,
		"Referer":         loginUrl,
		"User-Agent":      UA(),
	}
	c := g.Client()
	c.SetHeaderMap(headers)
	r, _ := c.Post(getUserCookieUrl, map[string]string{"lang": "chs", "appid": "300", "returnurl": "https://wqlogin2.jd.com/passport/LoginRedirect?state=" + Ntime + "&returnurl=//home.m.jd.com/myJd/newhome.action?sceneval=2&ufc=&/myJd/home.action", "source": "wq_passport"})
	defer r.Close()

	getCookies := r.GetCookieMap()

	//解析结果
	if j, err := gjson.DecodeToJson(r.ReadAllString()); err != nil {
		return 2, "错误！请检查网络！"
	} else {
		if j.GetInt("errcode") == 0 {
			var result string
			result += "pt_key=" + getCookies["pt_key"] + ";"
			result += "pt_pin=" + getCookies["pt_pin"] + ";"
			return 0, result
		} else {
			return 1, "授权登录未确认！"
		}
	}
}

//获得二维码
func getQrcode() interface{} {
	loc, _ := time.LoadLocation("Asia/Shanghai")

	Ntime := strconv.FormatInt(time.Now().In(loc).Unix(), 10)
	loginUrl := "https://plogin.m.jd.com/cgi-bin/mm/new_login_entrance?lang=chs&appid=300&returnurl=https://wq.jd.com/passport/LoginRedirect?state=" + Ntime + "&returnurl=https://home.m.jd.com/myJd/newhome.action?sceneval=2&ufc=&/myJd/home.action&source=wq_passport"
	headers := map[string]string{
		"Connection":      "Keep-Alive",
		"Content-Type":    "application/x-www-form-urlencoded",
		"Accept":          "application/json, text/plain, */*",
		"Accept-Language": "zh-cn",
		"Referer":         loginUrl,
		"User-Agent":      UA(),
	}
	c := g.Client()
	c.SetHeaderMap(headers)
	r, _ := c.Get(loginUrl)
	defer r.Close()

	var s_token string

	if j, err := gjson.DecodeToJson(r.ReadAllString()); err != nil {
		return nil
	} else {
		s_token = j.GetString("s_token")
	}

	cookies := parseCookie(r.RawResponse())
	if cookies == nil {
		return nil
	}

	c.SetCookieMap(cookies)

	Ntime = strconv.FormatInt(time.Now().In(loc).Unix(), 10)

	getQRUrl := "https://plogin.m.jd.com/cgi-bin/m/tmauthreflogurl?s_token=" + s_token + "&v=" + Ntime + "&remember=true"

	reqData := `{"lang": "chs", "appid": 300, "returnurl":"https://wqlogin2.jd.com/passport/LoginRedirect?state=` + Ntime + `&returnurl=//home.m.jd.com/myJd/newhome.action?sceneval=2&ufc=&/myJd/home.action", "source": "wq_passport"}`

	headers = map[string]string{
		"Connection":      "Keep-Alive",
		"Content-Type":    "application/x-www-form-urlencoded",
		"Accept":          "application/json, text/plain, */*",
		"Accept-Language": "zh-cn",
		"Referer":         loginUrl,
		"User-Agent":      UA(),
		"Host":            "plogin.m.jd.com",
	}
	c.SetHeaderMap(headers)
	res, _ := c.Post(getQRUrl, reqData)
	defer res.Close()

	var token string
	if j, err := gjson.DecodeToJson(res.ReadAllString()); err != nil {
		return nil
	} else {
		token = j.GetString("token")
	}

	cookies2 := parseCookie(res.RawResponse())
	okl_token := cookies2["okl_token"]
	qrCodeUrl := `https://plogin.m.jd.com/cgi-bin/m/tmauth?appid=300&client_type=m&token=` + token
	var rawCookie string
	for k, v := range cookies {
		rawCookie += k + "=" + v + ";"
	}
	png, _ := qrcode.Encode(qrCodeUrl, qrcode.Medium, 256)
	Fin := g.Map{"qrCode": png, "okl_token": okl_token, "cookies": rawCookie, "token": token}
	return Fin

}
