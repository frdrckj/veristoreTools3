## signature
### composition of original signature
    1、sift：
        Get all non empty request parameters except signature
    2、sort：
        Sort the sifted parameters in ascending order according to the ASCII code of the key value of the first character (alphabetical ascending order). If the same character is encountered, sort the parameters in ascending order according to the ASCII code of the key value of the second character, and so on.
    3、splice：
        Combine the sorted parameters and their values into the format of "parameter = parameter value", and concat these parameters with the character & and the generated string is the original signature.
        Note: if the parameter value is ether object or array, it needs to be converted to he JSON string first. The parameters in the generated JSON string also need to be sorted according to step 2. The array is sorted according to the data order.

### method
    HmacSHA256（AccessSecret, data）

### instance data
AccessKey：B5KJ89UL  
AccessSecret：cd26071795fb4a396f20057e6af06b30

#### example 1
```
{
	"accessKey": "B5KJ89UL",
	"timestamp": "1652421841733",
    "test'":"",
    "signature":"3447512232937DDCA6D7796718CA60495B714525C60696B70EE4BA9DA02A8E6D"
}
```
original text：accessKey=B5KJ89UL&timestamp=1652421841733  
signature：3447512232937DDCA6D7796718CA60495B714525C60696B70EE4BA9DA02A8E6D

#### example 2
```
{
    "subGroupIds":[
        1507197962226024449,
        1507198021411848200
    ],
    "groupName":"TestGroup",
    "accessKey":"B5KJ89UL",
    "signature":"67A184356ED75BBE6C1854B36B148E46FFB9FE8C18900B80404C0A25F33D597F",
    "terminalIds":[],
    "timestamp":1656469488197
}
```
original text：accessKey=B5KJ89UL&groupName=testGroup&subGroupIds=[1507197962226024449,1507198021411848200]&terminalIds=[]&timestamp=1656469488197  
signature：67A184356ED75BBE6C1854B36B148E46FFB9FE8C18900B80404C0A25F33D597F
