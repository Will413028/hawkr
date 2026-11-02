# Q7: Idempotency-Key on account creation
import sp, uuid
ctrl = {"requirement_collection":"application","fees":{"payer":"application"},"losses":{"payments":"application"},"stripe_dashboard":{"type":"none"}}
base={"country":"US","controller":ctrl,"capabilities":{"transfers":{"requested":True}},"business_type":"individual"}
k=str(uuid.uuid4())
s1,a1,h1=sp.call("POST","/v1/accounts",base,idem=k)
s2,a2,h2=sp.call("POST","/v1/accounts",base,idem=k)
print("first :",s1,a1.get("id"),"Idempotent-Replayed=",h1.get("Idempotent-Replayed"))
print("second:",s2,a2.get("id"),"Idempotent-Replayed=",h2.get("Idempotent-Replayed"))
print("same account id:",a1.get("id")==a2.get("id"))
diff=dict(base); diff["business_type"]="company"
s3,a3,h3=sp.call("POST","/v1/accounts",diff,idem=k)
sp.show("same key, different params",s3,a3)
print("headers on error: Idempotent-Replayed=",h3.get("Idempotent-Replayed"), "| Idempotency-Key header echoed:", "Idempotency-Key" in h3 or "idempotency-key" in {x.lower() for x in h3})
# different key, same params -> new account
s4,a4,_=sp.call("POST","/v1/accounts",base,idem=str(uuid.uuid4()))
print("different key, same params:",s4,"new account:",a4.get("id")!=a1.get("id"))
# key reuse across endpoints
s5,a5,_=sp.call("POST","/v1/accounts/"+a1["id"],{"business_profile":{"name":"x"}},idem=k)
sp.show("same key on a different endpoint",s5,a5)
# error responses are also cached: invalid params with a key, then retry
kb=str(uuid.uuid4()); bad=dict(base); bad["country"]="ZZ"
sa,oa,_=sp.call("POST","/v1/accounts",bad,idem=kb); sb,ob,hb=sp.call("POST","/v1/accounts",bad,idem=kb)
print("400 replay:",sa,sb,"Idempotent-Replayed=",hb.get("Idempotent-Replayed"))
