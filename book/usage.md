# 命令上传{#cmdline}

`curl -F file=@http-index-fs http://10.1.xx.60:8080/upload` 	


# WEB上传（浏览器打开）{#web}

`http://yourserver ip:8080/upload.html` 注意：不要使用127.0.0.1上传 	



## python版本: {#python}
```python
import requests
url = 'http://10.1.5.9:8080/upload'
files = {'file': open('report.xls', 'rb')}
options={'output':'json','path':'','scene':''} #参阅浏览器上传的选项
r = requests.post(url,data=options, files=files)
print(r.text)
```
## golang版本{#go}
```go
package main

import (
	"fmt"
	"github.com/astaxie/beego/httplib"
)

func main()  {
	var obj interface{}
	req:=httplib.Post("http://10.1.5.9:8080/upload")
	req.PostFile("file","filename")//注意不是全路径
	req.Param("output","json")
	req.Param("scene","")
	req.Param("path","")
	req.ToJSON(&obj)
	fmt.Print(obj)
}
```
## java版本{#java}
依赖(这里使用了hutool工具包,更简便)
```xml
<dependency>
	<groupId>cn.hutool</groupId>
	<artifactId>hutool-all</artifactId>
	<version>4.4.3</version>
</dependency>
```
上传代码
```java
public static void main(String[] args) {
    //文件地址
    File file = new File("D:\\git\\2.jpg");
    //声明参数集合
    HashMap<String, Object> paramMap = new HashMap<>();
    //文件
    paramMap.put("file", file);
    //输出
    paramMap.put("output","json");
    //自定义路径
    paramMap.put("path","image");
    //场景
    paramMap.put("scene","image");
    //上传
    String result= HttpUtil.post("http://xxxxx:xxxx/upload", paramMap);
    //输出json结果
    System.out.println(result);
}
```
[更多语言请参考](doc/upload.md)

