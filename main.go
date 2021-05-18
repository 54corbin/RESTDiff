package main

import (
	"bufio"
	"bytes"
	"container/list"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/antlabs/pcurl"
	jd "github.com/josephburnett/jd/lib"
)

type command struct {
	apiName string
	//vvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvvv

	leftCurl     string
	leftParms    *list.List
	leftResp     *list.List
	leftDuration *list.List
	//====================================
	rightCurl     string
	rightParms    *list.List
	rightResp     *list.List
	rightDuration *list.List

	//vvvvvvvvvvvvvvvvvvv
	report *list.List
}

var set = flag.Bool("set", false, "指明待比较json中的数组是有序的")

func main() {

	var file = flag.String("f", "", "存储curl命令的文件路径")
	var outPath = flag.String("o", "", "存储运行结果的目录")

	flag.Parse()

	if *file == "" {
		fmt.Println("请用 -f 指定 存放curl 命令的文件")
		os.Exit(-1)
	}
	f, err := os.Open(*file)
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	defer f.Close()
	if *outPath == "" {
		*outPath = filepath.Dir(f.Name())+string(os.PathSeparator)
	}
	//读取配置文件构造命令对象数组
	cmds := constructCmds(f)

	fmt.Println("接口总数：", cmds.Len())

	mwg := new(sync.WaitGroup)
	mwg.Add(cmds.Len())
	execute(cmds, mwg)

	//等待所有请求执行完毕
	mwg.Wait()

	//生成报告文件
	generateReport(cmds, outPath, f)

	fmt.Println("done....")

}

func execute(cmds *list.List, mwg *sync.WaitGroup) {
	progress := 0
	for cmd := cmds.Front(); cmd != nil; cmd = cmd.Next() {
		progress++
		go func(cmdVal *command, p int32) {

			defer mwg.Done()
			wg := new(sync.WaitGroup)
			wg.Add(2)
			fmt.Println("接口[", cmdVal.apiName, "]开始发起请求...")
			go doBatchCurl(wg, cmdVal.leftParms, cmdVal.leftCurl, cmdVal.leftResp, cmdVal.leftDuration)
			go doBatchCurl(wg, cmdVal.rightParms, cmdVal.rightCurl, cmdVal.rightResp, cmdVal.rightDuration)
			wg.Wait()
			fmt.Println("接口[", cmdVal.apiName, "]开始比对...")
			doBatchCompare(cmdVal.leftResp, cmdVal.rightResp, cmdVal.report)
			fmt.Printf("接口[%s] 对比结束...%d/%d\n", cmdVal.apiName, p, cmds.Len())
		}((cmd.Value).(*command), int32(progress))

	}
}


//生成报告文件
func generateReport(cmds *list.List, outPath *string, f *os.File) {
	for cmd := cmds.Front(); cmd != nil; cmd = cmd.Next() {
		cmdval := (cmd.Value).(*command)
		reportPath := filepath.Join(*outPath, cmdval.apiName)
		os.Mkdir(reportPath, os.ModePerm)

		leftResp := cmdval.leftResp.Front()
		rightResp := cmdval.rightResp.Front()
		leftParm := cmdval.leftParms.Front()
		rightParm := cmdval.rightParms.Front()
		leftDuration := cmdval.leftDuration.Front()
		rightDuration := cmdval.rightDuration.Front()

		if cmdval.leftResp.Len() != cmdval.rightResp.Len() || cmdval.leftDuration.Len() != cmdval.rightDuration.Len() || cmdval.leftResp.Len() != cmdval.rightDuration.Len() {
			fmt.Printf("结果集大小不匹配(respL:%d,respR:%d,durationL:%d,durationR:%d) 无法生成报告!!!\n", cmdval.leftResp.Len(), cmdval.rightResp.Len(), cmdval.leftDuration.Len(), cmdval.rightDuration.Len())
			os.Exit(-1)
		}

		for report := cmdval.report.Front(); report != nil; report = report.Next() {

			respName := strings.ReplaceAll(leftParm.Value.(string), " ", "_") + "_" + strings.ReplaceAll(rightParm.Value.(string), " ", "_")
			respFile := filepath.Join(reportPath, respName+".txt")

			reportName :=  strings.ReplaceAll(leftParm.Value.(string), " ", "_") + "_" + strings.ReplaceAll(rightParm.Value.(string), " ", "_")+"@report"
			reportFile := filepath.Join(reportPath, reportName+".txt")
			reportFile = strings.ReplaceAll(reportFile, "@@", "")
			reportFile = strings.ReplaceAll(reportFile, "encode", "")

			fmt.Println("输出报告：", reportFile)
			rep, err := os.Create(reportFile)
			defer f.Close()
			if err != nil {
				fmt.Println(err)
			}
			rep.Write([]byte(report.Value.(string)))

			fmt.Println("输出原始接口返回：", respFile)
			res, err := os.Create(respFile)
			defer f.Close()
			if err != nil {
				fmt.Println(err)
			}

			res.Write([]byte(fmt.Sprintf("=======================[耗时：%d ms \t 参数:%s]=============================\n", leftDuration.Value.(time.Duration).Milliseconds(),leftParm.Value.(string))))

			var ltmp bytes.Buffer
			err = json.Indent(&ltmp, []byte(leftResp.Value.(string)), "", "\t")
			if nil != err {
				fmt.Print("格式化失败")
				res.Write([]byte(leftResp.Value.(string)))
			} else {

				res.Write(ltmp.Bytes())
			}

			res.Write([]byte(fmt.Sprintf("\n\n=======================[耗时：%d ms \t 参数:%s]=============================\n", rightDuration.Value.(time.Duration).Milliseconds(),rightParm.Value.(string))))

			var rtmp bytes.Buffer
			err = json.Indent(&rtmp, []byte(rightResp.Value.(string)), "", "\t")
			if nil != err {
				fmt.Print("格式化失败")
				res.Write([]byte(rightResp.Value.(string)))
			} else {
				res.Write(rtmp.Bytes())
			}

			rightResp = rightResp.Next()
			leftResp = leftResp.Next()
			leftParm = leftParm.Next()
			rightParm = rightParm.Next()
			leftDuration = leftDuration.Next()
			rightDuration = rightDuration.Next()
		}

	}
}

//批量比较所有返回结果
func doBatchCompare(left *list.List, right *list.List, report *list.List) {

	if nil == left || nil == right || left.Len() != right.Len() {
		fmt.Println("左右结果集大小不匹配")
		report.PushBack("左右结果集大小不匹配")
		return
	}

	rightEle := right.Front()
	for leftEle := left.Front(); leftEle != nil; leftEle = leftEle.Next() {
		if leftEle == nil || rightEle == nil {
			fmt.Println("结果集为空无法对比")
			report.PushBack("左右结果集为空无法对比")
			return
		}

		lj := new(map[string]interface{})
		rj := new(map[string]interface{})

		LoadJsonFromString(leftEle.Value.(string), lj)
		LoadJsonFromString(rightEle.Value.(string), rj)

		diff, hasDiff := JsonCompare(*lj, *rj, -1)

		if hasDiff {
			fmt.Println("比对结果:", diff)
			report.PushBack(diff)
		} else {

			diff = "No differents"
			fmt.Println("比对结果:", diff)
			report.PushBack(diff)
		}
		rightEle = rightEle.Next()

		// diff := compare(leftEle.Value.(string), rightEle.Value.(string))

	}

}

//读取文件构造请求/比对 对象数组
func constructCmds(file *os.File) *list.List {

	cmds := list.New()

	br := bufio.NewReader(file)

	for count := 0; ; {

		left, err := readLine(br)
		count++
		if err != nil && err == io.EOF {
			break
		}

		//忽略井号#开头的
		if !strings.Contains(left, "curl") || !strings.Contains(left, "@") || strings.HasPrefix(left, "#") {
			continue
		}
		count++
		right, err := readLine(br)
		if (err != nil && err == io.EOF) || !strings.Contains(right, "curl") || 2 != strings.Count(left, "@") || 2 != strings.Count(right, "@") {
			fmt.Println(left, " >>格式不正确 行号：", count)
			os.Exit(-1)
		}

		tmpL := strings.Split(left, "@")
		tmpR := strings.Split(right, "@")

		// fmt.Printf("%+v",tmpL)

		cmd := new(command)
		cmd.leftParms = list.New()
		cmd.leftDuration = list.New()
		cmd.leftResp = list.New()

		cmd.rightParms = list.New()
		cmd.rightDuration = list.New()
		cmd.rightResp = list.New()

		cmd.report = list.New()

		cmd.apiName = tmpL[0]
		cmd.leftCurl = tmpL[2]
		cmd.rightCurl = tmpR[2]

		paramsFilePathL := fmt.Sprint(filepath.Dir(file.Name()), string(os.PathSeparator), tmpL[1], ".txt")
		paramsFilePathR := fmt.Sprint(filepath.Dir(file.Name()), string(os.PathSeparator), tmpR[1], ".txt")
		readParms(paramsFilePathL, cmd.leftParms)
		readParms(paramsFilePathR, cmd.rightParms)
		// fmt.Printf("leftParm:%+v\nrightParm:%+v\n", *cmd.leftParms,*cmd.rightParms)
		cmds.PushBack(cmd)
	}
	return cmds
}

//用每个不同的参数依次发送请求
func doBatchCurl(wg *sync.WaitGroup, parms *list.List, curl string, respList *list.List, durationList *list.List) {
	defer wg.Done()
	if nil == parms {
		fmt.Printf("配置文件有误！！！\n 请求：%s\n参数：%+v\n", curl, *parms)
		os.Exit(-1)

	}
	for parmLine := parms.Front(); parmLine != nil; parmLine = parmLine.Next() {

		parms := strings.Split((parmLine.Value).(string), "@@")

		tmpCurl := curl
		for i, parm := range parms {
			if strings.HasPrefix(parm, "encode:") {
				parm = strings.TrimPrefix(parm, "encode:")
				tmpCurl = strings.Replace(tmpCurl, "$$$", url.QueryEscape(parm), i+1)
				continue
			}

			tmpCurl = strings.Replace(tmpCurl, "$$$", parm, i+1)
		}

		leftResp, ld := curlReq(tmpCurl)
		respList.PushBack(leftResp)
		durationList.PushBack(ld)
	}
}

//将参数文件按行存入list
func readParms(filePath string, li *list.List) {
	fmt.Println("读取参数文件：", filePath)

	f, err := os.Open(filePath)
	defer f.Close()
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	pfBr := bufio.NewReader(f)
	for line, err := readLine(pfBr); err == nil || err != io.EOF; line, err = readLine(pfBr) {
		if line == "" {
			continue
		}
		li.PushBack(line)
	}

}

//do curl http request
func curlReq(cmd string) (string, time.Duration) {

	start := time.Now()
	req, err := pcurl.ParseAndRequest(cmd)
	if err != nil {
		fmt.Printf("\n%s\n\t\tVVVVVVVVVVVVVVVVVVVVVV\n\t\t\t\tVVV\n%s\n\n", cmd, err)
		es := fmt.Sprintf("%v", err)
		return es, 0
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("\n%s\n\t\tVVVVVVVVVVVVVVVVVVVVVV\n\t\t\t\tVVV\n%s\n\n", cmd, err)
		es := fmt.Sprintf("%v", err)
		return es, 0
	}

	duration := time.Since(start)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	fmt.Printf("\n%s\n\t\tVVVVVVVVVVVVVVVVVVVVVV\n\t\t\t\tVVV\n%s\n\n", cmd, string(body))
	return string(body), duration
}

//从reader里读取一行
func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadBytes('\n')
	line = bytes.TrimRight(line, "\r\n")
	return string(line), err

}

/**
 *比较两个json字符串并将结果存入 outPath指定的文件
**/
func compare(lj string, rj string) string {

	a, err := jd.ReadJsonString(lj)
	if nil != err {
		es := fmt.Sprintf("%s\n\nerror：%s\n", lj, err)
		fmt.Println(es)
		return es
	}
	b, err2 := jd.ReadJsonString(rj)
	if nil != err2 {
		es := fmt.Sprintf("%s\n\nerror：%s\n", rj, err)
		fmt.Println(es)
		return es
	}

	var diff string
	if *set {
		//添加 -set 参数指明待比较json中的数组是无序的
		metadata := make([]jd.Metadata, 0)
		metadata = append(metadata, jd.SET)
		diff = a.Diff(b, metadata...).Render()
	} else {
		diff = a.Diff(b).Render()
	}

	return diff
}
