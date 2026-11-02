# Q3: hosted (Account Links) and embedded (Account Sessions) onboarding
import sp, time
sc = {"requirement_collection":"stripe","fees":{"payer":"application"},"losses":{"payments":"application"},"stripe_dashboard":{"type":"express"}}
s,ex,_=sp.call("POST","/v1/accounts",{"country":"US","controller":sc,"capabilities":{"card_payments":{"requested":True},"transfers":{"requested":True}}})
sp.show("express (requirement_collection=stripe) create",s,ex,["id","type","controller","charges_enabled","payouts_enabled","details_submitted"])
e=ex["id"]
sp.show("express state right after create",200,sp.state(sp.call("GET","/v1/accounts/"+e)[1]))
now=int(time.time())
def link(label, acct, d):
    s,o,_=sp.call("POST","/v1/account_links",dict(d,account=acct))
    ea=o.get("expires_at")
    print(f"--- {label} HTTP {s}")
    print({k:(v if k!="url" else "<url>") for k,v in o.items()}, "| expires_in_s =", (ea-now) if ea else None)
    if o.get("url"): print("url host/path shape:", o["url"].split("?")[0][:60])
    else: sp.show(label,s,o)
    return o
link("account_onboarding, express",e,{"type":"account_onboarding","refresh_url":"https://example.com/refresh","return_url":"https://example.com/return","collection_options":{"fields":"eventually_due"}})
link("account_onboarding, express, future_requirements=include",e,{"type":"account_onboarding","refresh_url":"https://example.com/refresh","return_url":"https://example.com/return","collection_options":{"fields":"currently_due","future_requirements":"include"}})
link("account_update, express (docs: Custom only)",e,{"type":"account_update","refresh_url":"https://example.com/refresh","return_url":"https://example.com/return"})
link("account_onboarding without return_url",e,{"type":"account_onboarding","refresh_url":"https://example.com/refresh"})
link("account_onboarding with http (non-https) return_url",e,{"type":"account_onboarding","refresh_url":"http://example.com/refresh","return_url":"http://example.com/return"})
# application-collected account
cc = {"requirement_collection":"application","fees":{"payer":"application"},"losses":{"payments":"application"},"stripe_dashboard":{"type":"none"}}
s,cu,_=sp.call("POST","/v1/accounts",{"country":"US","controller":cc,"capabilities":{"card_payments":{"requested":True},"transfers":{"requested":True}},"business_type":"individual"})
c=cu["id"]
link("account_onboarding on application-collected account",c,{"type":"account_onboarding","refresh_url":"https://example.com/refresh","return_url":"https://example.com/return"})
link("account_update on application-collected account",c,{"type":"account_update","refresh_url":"https://example.com/refresh","return_url":"https://example.com/return"})
# Account Sessions (embedded)
for label,acct in [("express",e),("application-collected",c)]:
    s,o,_=sp.call("POST","/v1/account_sessions",{"account":acct,"components":{"account_onboarding":{"enabled":True,"features":{"external_account_collection":True}}}})
    if s==200: print(f"--- account_session on {label}: HTTP {s} keys={sorted(o)} expires_in_s={o['expires_at']-now} components={list(o['components'])}")
    else: sp.show(f"account_session on {label}",s,o)
s,o,_=sp.call("POST","/v1/account_sessions",{"account":e,"components":{"account_management":{"enabled":True},"notification_banner":{"enabled":True}}})
print("--- account_session account_management+notification_banner:",s, o.get("error",{}).get("message","ok"))
# Hosted link validity: GET the link? only check HEAD semantic is out of scope.
for a in (e,c):
    print("state",a, sp.state(sp.call("GET","/v1/accounts/"+a)[1])["requirements"]["disabled_reason"])
