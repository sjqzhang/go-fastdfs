# 统一说明（重要）{#description}
```
从1.3.3开始，默认开启组(support_group_manage=true)功能（有更好的扩展性）
访问的url中都会带上group的名称，例如：
http://10.1.5.9:8080/这里是组名/upload
如:
http://10.1.5.9:8080/group1/upload
如果（support_group_manage=false）
url变为
http://10.1.5.9:8080/upload

```
# 命令上传{#cmdline}

`curl -F file=@http-index-fs http://10.1.xx.60:8080/group1/upload` 	


# WEB上传（浏览器打开）{#web}

`http://yourserver ip:8080/upload.html` 注意：不要使用127.0.0.1上传 	



## python版本: {#python}
```python
import requests
url = 'http://10.1.5.9:8080/group1/upload'
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
	req:=httplib.Post("http://10.1.5.9:8080/group1/upload")
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
    String result= HttpUtil.post("http://xxxxx:xxxx/group1/upload", paramMap);
    //输出json结果
    System.out.println(result);
}
```

## java版本流式上传{#java-stream}

Hutool-http、HttpClient、OkHttp3多种方式流式文件上传
由于有不少人问到上面的问题，现在本人总结了一个常用的几种http客户端文件流式上传的方式，相当于给自己做下记录，同时也给有这方面疑问的朋友一个借鉴。废话不多说，直接上代码吧。代码是基于springboot的maven工程。

Hutool-http方式
先在pom中添加hutool的依赖
```xml
 <dependency>
	 <groupId>cn.hutool</groupId>
	 <artifactId>hutool-all</artifactId>
	 <version>4.5.1</version>
 </dependency>
 ```
接着在Controller中代码示例
```java
    @RequestMapping("/upload")
    public String  upload(MultipartFile file) {
        String result = "";
        try {
            InputStreamResource isr = new InputStreamResource(file.getInputStream(),
                    file.getOriginalFilename());

            Map<String, Object> params = new HashMap<>();
            params.put("file", isr);
            params.put("path", "86501729");
            params.put("output", "json");
            String resp = HttpUtil.post(UPLOAD_PATH, params);
            Console.log("resp: {}", resp);
            result = resp;
        } catch (IOException e) {
            e.printStackTrace();
        }
        
        return result;
    }
```
HttpClient方式
pom依赖
```xml
<dependency>
	<groupId>org.apache.httpcomponents</groupId>
	<artifactId>httpclient</artifactId>
</dependency>
<dependency>
	<groupId>org.apache.httpcomponents</groupId>
	<artifactId>httpmime</artifactId>
</dependency>
```
接着在Controller中代码示例
```java
    @RequestMapping("/upload1")
    public String upload1(MultipartFile file) {
        String result = "";
        try {
            CloseableHttpClient httpClient = HttpClientBuilder.create().build();
            CloseableHttpResponse httpResponse = null;
            RequestConfig requestConfig = RequestConfig.custom()
                    .setConnectTimeout(200000)
                    .setSocketTimeout(2000000)
                    .build();
            HttpPost httpPost = new HttpPost(UPLOAD_PATH);
            httpPost.setConfig(requestConfig);
            MultipartEntityBuilder multipartEntityBuilder = MultipartEntityBuilder.create()
                    .setMode(HttpMultipartMode.BROWSER_COMPATIBLE)
                    .setCharset(Charset.forName("UTF-8"))
                    .addTextBody("output", "json")
                    .addBinaryBody("file", file.getInputStream(),
                            ContentType.DEFAULT_BINARY, file.getOriginalFilename());
            httpPost.setEntity(multipartEntityBuilder.build());
            httpResponse = httpClient.execute(httpPost);

            if (httpResponse.getStatusLine().getStatusCode() == 200) {
                String respStr = EntityUtils.toString(httpResponse.getEntity());
                System.out.println(respStr);
                result = respStr;
            }

            httpClient.close();
            httpResponse.close();
        } catch (Exception e) {
            e.printStackTrace();
        }
        return result;
    }
```
OkHttp3上传示例
pom文件依赖
```xml
 <dependency>
	 <groupId>com.squareup.okhttp3</groupId>
	 <artifactId>okhttp</artifactId>
	 <version>3.9.1</version>
 </dependency>
```
接着在Controller中代码示例
```java
    @RequestMapping("/upload2")
    public String upload2(MultipartFile file) {
        String result = "";
        try {
            OkHttpClient httpClient = new OkHttpClient();
            MultipartBody multipartBody = new MultipartBody.Builder().
                    setType(MultipartBody.FORM)
                    .addFormDataPart("file", file.getOriginalFilename(),
                            RequestBody.create(MediaType.parse("multipart/form-data;charset=utf-8"),
                                    file.getBytes()))
                    .addFormDataPart("output", "json")
                    .build();

            Request request = new Request.Builder()
                    .url(UPLOAD_PATH)
                    .post(multipartBody)
                    .build();

            Response response = httpClient.newCall(request).execute();
            if (response.isSuccessful()) {
                ResponseBody body = response.body();
                if (body != null) {
                    result = body.string();
                    System.out.println(result);
                }
            }
        } catch (Exception e) {
            e.printStackTrace();
        }

        return result;
    }
```
总结
上面给出了几个示例，是不是都挺简单的？通过这种方式，就可以在Controller中做中转了，还是挺方便的。顺便提一下，上面几种方式中，我个人觉得Hutool的是最简单的，最方便的，对于HttpClient而言，概念比较多，显得相对复杂，OkHttp也一样，不过比HttpClient显得优雅点。针对一般的并发量，个人觉得hutool的Http已经够用了，底层是基于jdk的HttpUrlConnection实现的。如果对性能有特殊要求的，可以考虑httpclient或者OKHttp，后两者相对而言，更推荐使用OkHttp。



