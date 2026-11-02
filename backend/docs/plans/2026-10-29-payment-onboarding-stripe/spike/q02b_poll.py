# Q2 follow-up: poll an account until verification settles (account id as env ACCT)
import sp, os, time
i=os.environ["ACCT"]
for n in range(12):
    s,a,_=sp.call("GET","/v1/accounts/"+i)
    r=a["requirements"]
    print(f"t+{n*10}s charges={a['charges_enabled']} payouts={a['payouts_enabled']} caps={a['capabilities']} pending={r['pending_verification']} due={r['currently_due']} dis={r['disabled_reason']}")
    if a["charges_enabled"] and a["payouts_enabled"]: break
    time.sleep(10)
sp.show("final", s, sp.state(a))
