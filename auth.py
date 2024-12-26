from flask import  Flask,request
app = Flask(__name__)
@app.route('/',methods=["GET", "POST"])
def index():
    print(request.headers)
    auth_token = request.headers.get("auth_token") # check auth_token here
    print(auth_token)
    if auth_token=='abc':
        return 'ok' #success
    else:
        return 'fail'
if __name__ == '__main__':
    app.run(host='0.0.0.0',debug=True)