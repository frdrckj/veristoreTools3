## application API

### List applications

**Description**

The Search application API, allows third-party systems to search for the latest version of the application page

**Request Parameters**

| Name      | Type    | Nullable | Description                             |
| :-------- | :------ | :------- | :-------------------------------------- |
| accessKey | String  | false    |                                         |
| timestamp | Long    | false    | timestamp(resolution of milliseconds).  |
| signature | String  | false    | signature                               |
| page      | Integer | true     | current page of the list<br/> default:1 |
| size      | Integer | true     | rows in a page<br/> default:10          |
| search    | String  | true     | by application name                     |

**Request Example**
sp
```
POST /url
HTTP/1.1
host: /v1/tps/app/list
content-type:application/json; charset=utf-8; Accept-Language=en-GB;
{
    "accessKey":"FA1D66ED",
    "timestamp":"1654671444092",
    "signature":"f128f968aa584c3b22d93539a73c934a73e34937e2db8d473230cfc36da9f105",
    "page":1,
    "size":10
}
```

**Response Parameters**

| Name        | Type   | Nullable | Description         |
| :---------- | :----- | :------- | :------------------ |
| id          | String | true     | application ID      |
| createTime  | String | true     | the creation time   |
| downloads   | String | true     | downloads           |
| icon        | String | true     | icon uri            |
| model       | String | true     | model               |
| name        | String | true     | application name    |
| packageName | String | true     | package name        |
| size        | String | true     | file size           |
| version     | String | true     | application version |

**Response Example**

```
{
	"code": "200",
	"desc": "Query was successful",
	"data": {
		"pages": 1,
		"total": 5,
		"list": [
			{
				"id": "1532203368274817026",
				"name": "VeriStore",
				"packageName": "cn.verifone.veristore",
				"icon": "http://121.41.13.18:8280/market/opt/store/tempDir/0eedba10-9c94-417f-92a5-ed33cec6d95f.png",
				"version": "2.1.0.3",
				"size": "13.05MB",
				"model": "X990 v4,X990 v2",
				"createTime": "1654140724000",
				"downloads": "2"
			},
			{
				"id": "1532179175307046914",
				"name": "VeristoreGuideDemo",
				"packageName": "com.verifone.veristoreguidedemo",
				"icon": "http://121.41.13.18:8280/market/opt/store/tempDir/7370444b-d9c4-474e-8b55-cdcb1e3889c1.png",
				"version": "1.0",
				"size": "4.07MB",
				"model": "X550,X990 v1,X970,X990 v4",
				"createTime": "1654134956000",
				"downloads": "3"
			},
			{
				"id": "1531896659434106882",
				"name": "KingPower",
				"packageName": "com.thailand.kingpower",
				"icon": "http://121.41.13.18:8280/system/opt/store/manage/upload/app/appFile/6f3faddb-83e1-4f3b-b240-14e72861a3ba.jpg",
				"version": "1.1.16",
				"size": "5.28MB",
				"model": "X990 v1,X990 v4,X990 v2",
				"createTime": "1654067600000",
				"downloads": "4"
			},
			{
				"id": "1529753895330197506",
				"name": "BIJLIPAY",
				"packageName": "com.verifone.india.bp.presentation",
				"icon": "http://121.41.13.18:8280/market/opt/store/tempDir/996ae002-a818-49f2-91c4-2b7dc4fd071a.png",
				"version": "1.0.0.36",
				"size": "9.67MB",
				"model": "X990 v1,X990 v4,X990 v2",
				"createTime": "1653556724000",
				"downloads": "20"
			},
			{
				"id": "1529749459107131394",
				"name": "KBXPayment",
				"packageName": "com.vfi.android.payment.kbank",
				"icon": "http://121.41.13.18:8280/market/opt/store/tempDir/61d27c5c-bf43-4064-8daa-87cf7f2b41bd.png",
				"version": "1.6.53",
				"size": "11.52MB",
				"model": "X990 v1,X990 v4,X990 v2",
				"createTime": "1653555667000",
				"downloads": "8"
			}
		]
	},
	"signature": "F0B70C0E6C79FF7B014187FEA9D21DC88DCA8E9824F2039F5BFA327DF4F274DF"
}
```

### application detail

**Description** 

Application Details API, allows third-party systems to obtain application details.

**Request Parameters**

| Name        | Type   | Nullable | Description         |
| :---------- | :----- | :------- | :------------------ |
| accessKey   | String | false    |                     |
| timestamp   | Long   | false    | timestamp           |
| signature   | String | false    | signature           |
| packageName | String | false    | package name        |
| version     | String | false    | application version |
| appId       | String | false    | application ID      |



**Request Example**

```
POST /url
HTTP/1.1
host: /v2/tps/app/detail
content-type:application/json; charset=utf-8; Accept-Language=en-GB;

{
	"accessKey":"1989346D",
	"appId":"1827908599662252034",
	"packageName":"cn.verifone.veristore",
	"signature":"628BCB1EAE9B1BCEB7C23B7ACFE3B552051BA246296508AFAF9E75A15ED16FE5",
	"timestamp":1724643383792,
	"version":"2.5.2"
}
```

**Response Parameters**

| Name              | Type    | Nullable | Description                                      |
| :---------------- | :------ | :------- | :----------------------------------------------- |
| id                | String  | true     | application ID                                   |
| name              | String  | true     | application name                                 |
| packageName       | String  | true     | package name                                     |
| icon              | String  | true     | icon url                                         |
| version           | String  | true     | application version                              |
| size              | String  | true     | package size                                     |
| uninstallMode     | Integer | true     | whether can be uninstalled<br/> 0:no；<br/>1:yes |
| developer         | String  | true     | developer                                        |
| description       | String  | true     | description                                      |
| newInfo           | String  | true     | new features                                     |
| appScreenshotList | List    | true     | screenshot uri                                   |
| androidVersions   | List    | true     | supported android versions                       |
| modelList         | List    | true     | supported models                                 |
| categoryName      | String  | true     | category name                                    |
| downloads         | String  | true     | downloads                                        |
| updateTime        | String  | true     | the update time                                  |
| templates         | List    | true     | parameters‘ templates                            |

  *parameters‘ templates*

| Name         | Type    | Nullable | Description                                    |
| :----------- | :------ | :------- | :--------------------------------------------- |
| id           | String  | true     | template ID                                    |
| templateName | String  | true     | template name                                  |
| templateType | Integer | true     | template type<br/> 0:non-default;<br/>1:default |

**Response Example**

```
{
	"code":"200",
	"data":{
		"androidVersions":[
			"Android7",
			"Android10"
		],
		"appScreenshotList":[
			"http://fzjftest.vfcnserv.com:28280/market/opt/store/tempDir//14c3ab7c-4ae5-4c85-8dd1-bf461ae41066.png"
		],
		"categoryName":"Retail",
		"description":"veriStore",
		"developer":"vfcn",
		"downloads":"0",
		"icon":"http://fzjftest.vfcnserv.com:28280/market/opt/store/tempDir/75b3c08d-8c6f-4576-935c-b6ecaa33a81b.png",
		"id":"1827908599662252034",
		"modelList":[
			"verifone&X990 v2",
			"verifone&X990 v4"
		],
		"name":"VeriStore",
		"newInfo":"V2.5.2",
		"packageName":"cn.verifone.veristore",
		"size":"10.85MB",
		"templates":[
			{
				"id":"1827912301978587138",
				"templateName":"20240826113345_terminalParameter_1715593918784.xml",
				"templateType":1
			}
		],
		"uninstallMode":0,
		"updateTime":"1724643287000",
		"version":"2.5.2"
	},
	"desc":"Query was successful",
	"signature":"C973049B71D6AB0F91385846C3881AD031F570CF1017D17DA792ADD24961E18C"
}
```