#!/bin/bash
curl -k --data "username=user100" --data "password=password" -H"Cookie: X-Uaa-Csrf=value" --data "X-Uaa-Csrf=value" https://perfzone9.login.identity.cf-app.com/login.do -v 2>&1 |grep Set-Cookie > http-response.txt
export JSESSION_COOKIE=`cat http-response.txt |grep JSESSIONID |awk '{print $3}'`
export VCAP_COOKIE=`cat http-response.txt |grep VCAP_ID |awk '{print $3}'`

echo "UAA: $JSESSION_COOKIE"
echo "VCAP: $VCAP_COOKIE"

rm -f http-response.txt

#curl -H "Cookie: $JSESSION_COOKIE" -H "Cookie: $VCAP_COOKIE" "https://perfzone9.login.identity.cf-app.com/oauth/authorize?response_type=code&state=1234&client_id=client1005&redirect_uri=http://localhost" -k -v
curl -H "Cookie: $JSESSION_COOKIE $VCAP_COOKIE" "https://perfzone9.login.identity.cf-app.com/oauth/authorize?response_type=code&state=1234&client_id=client1005&redirect_uri=http://localhost" -k -v

go run src/github.com/cloudfoundry/uaa-hey/hey.go -n 1 -c 1 -t 5 -s 302 -H "Cookie: $JSESSION_COOKIE $VCAP_COOKIE" "https://perfzone9.login.identity.cf-app.com/oauth/authorize?response_type=code&state=1234&client_id=client1005&redirect_uri=http://localhost&tools"
