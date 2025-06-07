# README

## Paths  

### Swagger  

Code:  
`/home/kali/code/toolkit`  

Swagger:  
`/home/kali/.config/toolkit/swagger`  


## ZIP up the toolkit folder  
```
zip -r toolkit_v.zip toolkit
```  

## Add in-scope rule  
```
curl -X POST http://localhost:8778/api/scope-rules \
-H "Content-Type: application/json" \
-d '{
    "target_id": 1,
    "item_type": "domain",
    "pattern": "*.api.example.com",
    "is_in_scope": true,
    "description": "All API subdomains"
}'
```  

## Get all scope rules for target  

```
curl -X GET "http://localhost:8778/api/scope-rules?target_id=1"
```  

## Delete scope rule  

```
curl -X DELETE http://localhost:8778/api/scope-rules/2
```  
