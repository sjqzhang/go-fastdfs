# 按文件指纹认证{#fingerprint}
### 注意如果是海量文件建议不要使用这种方式认证，随着文件的增加，认证性能有所下降。

```
1、上传文件（json），此时获得文件上传指纹（md5值）
2、计算token, 计算的方式： token=md5(文件指纹+时间戳)
3、下载文件时，在url中带上token

例子：
1、上传文件
{
  "url": "http://127.0.0.1:8080/group1/haystack/5/208,0,58602,.png",
  "md5": "2b33b60980b1a37454f008daf7e5d558",
  "path": "/group1/haystack/5/208,0,58602,.png",
  "domain": "http://127.0.0.1:8080",
  "scene": "default",
  "size": 58602,
  "mtime": 1566524934,
  "scenes": "default",
  "retmsg": "",
  "retcode": 0,
  "src": "/group1/haystack/5/208,0,58602,.png"
}
2、计算token(注意计算时是没有中间加号的) 
token=md5(2b33b60980b1a37454f008daf7e5d558 + 1566783964)
token=6d70bf9607b692f00d27a3c554bb64d0
3、下载文件



```


# 按google认证码认证{#google}
### 如果你对google认证较为熟悉，建议使用这种认证，性能好，安全性也更高。

![google认证](https://raw.githubusercontent.com/sjqzhang/go-fastdfs/master/doc/authentication.png)


# 用户自定义认证{#custom}

### 如果你是海量文件，又需要有自己的一些验证逻辑，建议使用自定义认证。

![google认证](https://raw.githubusercontent.com/sjqzhang/go-fastdfs/master/doc/authentication2.png)

