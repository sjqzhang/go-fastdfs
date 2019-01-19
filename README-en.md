# distributed file server go-fastdfs (like fastdfs) is better than fastdfs in operation and maintenance management, and more humanized
- Supporting curl command upload
- Support browser upload
- Support HTTP Download
- Support multi-machine automatic synchronization
Class fastdfs
- High performance (using leveldb as a kV library)
- High reliability (extremely simple design, using mature components)
Advantages and disadvantages
- No dependency (single file)
- Automatic Synchronization
- Failure Auto-repair
- Easy maintenance by genius catalogue
- Support different scenarios
- Automatic de-duplication of files
- Support directory customization
- Support for retaining the original file name
- Support for automatic generation of unique file names
- Support browser upload
- Support for viewing cluster file information
- Support cluster monitoring mail alerts
- Support token download token = MD5 (file_md5 + timestamp)
- Simple operation and maintenance, with only one role (unlike fastdfs, which has three roles, Tracker Server, Storage Server, Client), and automatic configuration generation
- Peer to peer for each node (simplified operation and maintenance)
- All nodes can read and write at the same time
# Start the server (compiled, [downloaded](https://github.com/sjqzhang/fastdfs/releases)experience)
`. / fileserver`
Configuration automatic generation (conf/cfg.json)
```json
{
	"addr": ":8080",
	"peers": ["http://10.1.xx.2:8080", "http://10.1.xx.5:8080", "http://10.1.xx.60:8080"],
	"group": "group1",
	"refresh_interval": 120,
	"rename_file": false,
	"enable_web_upload": true,
	"enable_custom_path": true,
	"download_domain": "",
	"scenes": [],
	"default_scene": "default",
	"show_dir": true,
	"mail": {
		"user": "abc@163.com",
		"password": "abc",
		"host": "smtp.163.com:25"
	},
	"alram_receivers": [],
	"alarm_url": "",
	"download_use_token": false,
	"download_token_expire": 600,
	"auto_repair": true
}
```
Command upload
`curl-F file=@http-index-fs http://10.1.xx.60:8080/upload`
# WEB Upload (Browser Open)
`http://127.0.0.1:8080`
# Code Upload (Options See Browser Upload)
```python
import requests
url = 'http://127.0.0.1:8080/upload'
files = {'file': open('report.xls', 'rb')}
options={'output':'json','path':'','scene':''} # refer to browser upload options
r = requests.post(url, files=files)
print(r.text)
```
# If you have any questions, please [click on feedback](https://github.com/sjqzhang/go-fastdfs/issues/new)
Q&amp;A
- Can files already stored in fastdfs be migrated to go fastdfs?
```
The answer is yes. The problem you worry about is path change. Go fast DFS considers this for you.
Curl-F file=@data/00/00/_78HAFwyvN2AK6ChAAHg8gw80FQ213.jpg-F path=M00/00/00/http://127.0.0.1:8080/upload
Similarly, you can migrate all files with one line of command
CD fastdfs/data &amp; &amp; find-type f | xargs-n 1-I {} curl-F file =@data/{}-F path = M00/00/00/ http://127.0.0.1:8080/
The above commands can be overridden
You can write some simple scripts for migration
```
- Need nginx be installed?
```
You can either not install it, or you can choose to install it.
Go fastdfs itself is a high performance web file server.
```
- How to view cluster file information?
```
Http://10.1.xx.60:8080/stat
What if there is a statistical error?
Please delete stat.json file in the data directory and restart the service. Please recalculate the number of files automatically.
```
- How reliable can it be used in the production environment?
```
The project has been widely used in the production environment, such as fear of unsatisfactory
It can be used for pressure testing of its various characteristics before use.
Questions can be asked directly.
```
- Can you set up multiple servers in a machine department?
```
No, the high availability of clusters has been considered at the beginning of the design. In order to ensure the true availability of clusters, different IPs must be used.
Error "peers": ["http://127.0.0.1:8080", "http://127.0.0.1:8081", "http://127.0.1:8082"]
Correct "peers": ["http://10.0.0.3:8080", "http://10.0.0.4:8080", "http://10.0.0.5:8082"]
```
- What if the file is out of sync?
```
Normally, the cluster will repair files automatically and synchronously every hour. (poor performance, it is recommended to turn off automatic repair in mass cases)
What about the abnormal situation?
Answer: Manual Synchronization
Http://172.16.70.123:7080/sync?Date=20190117&amp;force=1
Parametric description: date represents the data force of the synchronization day 1. Indicates whether to force all (poor performance) files on the synchronization day, and 0. Indicates only files that failed to synchronize.
In case of asynchrony:
1) Originally running N sets, now suddenly adding one becomes N + 1 sets.
2) The original operation of N sets, a machine problems, into N-1 sets
```
- Does not file synchronization affect access?
```
Answer: It won't affect. It will automatically fix asynchronous files when it is not accessible.
```
- How to measure?
```
gen_file.py is used to generate a large number of files (note that if you want to generate a large file, you multiply it in your content)
# -*- coding: utf-8 -*-
import os
j=0
for i in range(0,1000000):
    if i%1000==0:
        j=i
        os.system('mkdir %s'%(i))
    with open('%s/%s.txt'%(j,i),'w+') as f:
        f.write(str(i)*1024)
Then the pressure measurement is done with benchmark.py.
It can also be measured by multiple computers at the same time. All nodes can be read and written at the same time.
```
