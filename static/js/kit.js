(function($){
    var kit = function(){}
    kit.prototype = {
        getIconName: function(suffix) {
            var iconName;
            switch(suffix) {
                //图片
                case "jpg":
                    iconName = "img";break;
                case "png":
                    iconName = "img";break;
                case "jpeg":
                    iconName = "img";break;
                case "gif":
                    iconName = "img";break;
                case "psd":
                    iconName = "img";break;
                //压缩包
                case "rar":
                    iconName = "zip";break;
                case "zip":
                    iconName = "zip";break;
                case "7z":
                    iconName = "zip";break;
                case "tar":
                    iconName = "zip";break;
                case "gz":
                    iconName = "zip";break;
                //ppt
                case "ppt":
                    iconName = "ppt";break;
                case "pptx":
                    iconName = "ppt";break;
                //pdf
                case "pdf":
                    iconName = "pdf";break;
                //word
                case "doc":
                    iconName = "word";break;
                case "docx":
                    iconName = "word";break;
                //excel
                case "xls":
                    iconName = "excel";break;
                case "xlsx":
                    iconName = "excel";break;
                //歌曲
                case "wave":
                    iconName = "music";break;
                case "mp3":
                    iconName = "music";break;
                case "mpeg-4":
                    iconName = "music";break;
                case "aac":
                    iconName = "music";break;
                case "mpeg":
                    iconName = "music";break;
                //文本
                case "txt":
                    iconName = "txt";break;
                //视频
                case "avi":
                    iconName = "video";break;
                case "mp4":
                    iconName = "video";break;
                case "3gp":
                    iconName = "video";break;
                case "rmvb":
                    iconName = "video";break;
                case "flv":
                    iconName = "video";break;
                //exe
                case "exe":
                    iconName = "exe";break;
                //脚本文件
                case "sh":
                    iconName = "shell";break;
                case "bat":
                    iconName = "shell";break;
                //java
                case "java":
                    iconName = "java";break;
                //go
                case "go":
                    iconName = "go";break;
                //css
                case "css":
                    iconName = "css";break;
                //html
                case "html":
                    iconName = "html";break;
                //js
                case "js":
                    iconName = "js";break;
                //python
                case "py":
                    iconName = "python";break;
                //其他
                default:
                    iconName = "other";break;
            }
            return iconName;
        },
        getFileType: function(suffix) {
            var fileType;
            if(suffix == "jpg" ||suffix == "png" ||suffix == "jpeg" ||suffix == "gif" ||suffix == "psd"){
                fileType = "image";
            }else if(suffix == "rar" ||suffix == "zip" ||suffix == "7z" ||suffix == "tar" ||suffix == "gz"){
                fileType = "zip";
            }else if(suffix == "ppt" ||suffix == "pptx"){
                fileType = "ppt";
            }else if(suffix == "doc" ||suffix == "docx"){
                fileType = "word";
            }else if(suffix == "xls" ||suffix == "xlsx"){
                fileType = "excel";
            }else if(suffix == "wave" ||suffix == "mp3" ||suffix == "mpeg-4" ||suffix == "aac" ||suffix == "mpeg"){
                fileType = "song";
            }else if(suffix == "txt"){
                fileType = "txt";
            }else if(suffix == "avi" ||suffix == "mp4" ||suffix == "3gp" ||suffix == "rmvb" ||suffix == "flv"){
                fileType = "video";
            }else{
                fileType = "other";
            }
            return fileType;
        }
    }
    window.kit = new kit();
})(window.jQuery);