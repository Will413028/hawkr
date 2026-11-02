import sp, sys, time, json, subprocess, os
log, listenlog = sys.argv[1], sys.argv[2]
MINE=None
def deliveries():
    rows=[json.loads(l) for l in open(log)] if os.path.exists(log) else []
    return [r for r in rows if r["account"]==MINE]
cc = {"requirement_collection":"application","fees":{"payer":"application"},"losses":{"payments":"application"},"stripe_dashboard":{"type":"none"}}
s,a,_=sp.call("POST","/v1/accounts",{"country":"US","controller":cc,"capabilities":{"transfers":{"requested":True}},"business_type":"individual"})
i=a["id"]; MINE=i; print("account",i)
# rapid sequence of updates -> observe ordering
t0=time.time()
for n in range(5):
    sp.call("POST","/v1/accounts/"+i,{"metadata":{"seq":str(n)}})
time.sleep(8)
d=deliveries()
print(f"--- {len(d)} deliveries after create + 5 rapid updates")
for r in d: print(json.dumps({k:r[k] for k in ("recv","event","type","created","request_id","livemode","verify","altered_body_verify","altered_why","stale_ts_verify","stale_why","sig_header_parts","api_version")}))
au=[r for r in d if r["type"]=="account.updated"]
print("account.updated count:",len(au),"| all verified OK:",all(r["verify"] for r in d),"| every altered body rejected:",not any(r["altered_body_verify"] for r in d),"| every stale-timestamp rejected:",not any(r["stale_ts_verify"] for r in d))
print("order of delivery equals order of event.created:",[r["created"] for r in d]==sorted(r["created"] for r in d),"| created values:",[r["created"]-d[0]["created"] for r in d])
# event retrieval: ids and ordering
s,evs,_=sp.call("GET","/v1/events",{"limit":20,"type":"account.updated"},headers={"Stripe-Account":i})
print("Events API (Stripe-Account header) HTTP",s,"ids newest-first equal reverse of delivery order:",[e["id"] for e in evs.get("data",[])]==[r["event"] for r in reversed(au)])
# resend
target=au[-1]["event"]; n0=len(d)
env=dict(os.environ)
p=subprocess.run(["/opt/homebrew/bin/stripe","events","resend",target,"--account",i,"--api-key",sp.KEY],capture_output=True,text=True)
out=(p.stdout+p.stderr)
print("--- stripe events resend (no endpoint flag) rc=",p.returncode,"\n"+out[-700:].replace(sp.KEY,"<key>"))
time.sleep(6)
d2=deliveries(); new=d2[n0:]
print("deliveries after resend:",len(new),[ ("same event id as original" if r["event"]==target else "different id", r["type"], "sig verified" if r["verify"] else "sig FAIL") for r in new])
# tolerance of the CLI listen forward: signature timestamp
print("--- listen log (secret masked by scrub):"); 
print(open(listenlog).read()[-800:])
sp.call("DELETE","/v1/accounts/"+i)
