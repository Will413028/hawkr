# Q6: external bank account fingerprint across connected accounts
import sp
cc = {"requirement_collection":"application","fees":{"payer":"application"},"losses":{"payments":"application"},"stripe_dashboard":{"type":"none"}}
bank={"object":"bank_account","country":"US","currency":"usd","routing_number":"110000000","account_number":"000123456789"}
other={"object":"bank_account","country":"US","currency":"usd","routing_number":"110000000","account_number":"000111111116"}
ids=[]
for n in range(2):
    s,a,_=sp.call("POST","/v1/accounts",{"country":"US","controller":cc,"capabilities":{"transfers":{"requested":True}},"business_type":"individual","external_account":bank})
    ids.append(a["id"])
    if s!=200: sp.show("create ERROR",s,a)
fps=[]
for i in ids:
    s,o,_=sp.call("GET",f"/v1/accounts/{i}/external_accounts",{"object":"bank_account"})
    b=o["data"][0]; fps.append(b["fingerprint"])
    print(i,"bank fingerprint(masked, stable tag)=","fp_"+__import__("hashlib").sha256(b["fingerprint"].encode()).hexdigest()[:4],"| keys:",sorted(k for k in b if k in("fingerprint","last4","bank_name","status","default_for_currency")), "status=",b["status"])
print("fingerprint equal across two accounts, same bank:",fps[0]==fps[1])
# a third account, a different account number
s,a,_=sp.call("POST","/v1/accounts",{"country":"US","controller":cc,"capabilities":{"transfers":{"requested":True}},"business_type":"individual","external_account":other}); 
o=sp.call("GET",f"/v1/accounts/{a['id']}/external_accounts",{"object":"bank_account"})[1]["data"][0]
print("different account number fingerprint tag=","fp_"+__import__("hashlib").sha256(o["fingerprint"].encode()).hexdigest()[:4],"equal to first:",o["fingerprint"]==fps[0])
# same account added twice to ONE account
s,o2,_=sp.call("POST",f"/v1/accounts/{ids[0]}/external_accounts",{"external_account":bank})
print("attach same bank again to same account:",s,(o2.get("error") or {}).get("message"))
# is fingerprint stable across a change to the same number after deletion/recreate?
# token-based: does a btok (client-side token) give same fingerprint?
s,t,_=sp.call("POST","/v1/tokens",{"bank_account":{"country":"US","currency":"usd","routing_number":"110000000","account_number":"000123456789","account_holder_type":"individual"}})
s,o3,_=sp.call("POST",f"/v1/accounts/{a['id']}/external_accounts",{"external_account":t.get("id")})
print("token path fingerprint equal to first:", o3.get("fingerprint")==fps[0], "status",s)
# fingerprint on a company-type bank with different holder type
print("fingerprint visible with platform key on retrieve of account object external_accounts list: ", "fingerprint" in sp.call("GET","/v1/accounts/"+ids[1])[1]["external_accounts"]["data"][0])
