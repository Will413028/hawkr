# Q2 follow-up: can SG proof_of_liveness be satisfied through the API?
import sp, os, time
i=os.environ["ACCT"]
for label, d in [("proof_of_liveness=test doc token", {"individual":{"verification":{"proof_of_liveness":"file_identity_document_success"}}}),
                 ("additional_document front", {"individual":{"verification":{"additional_document":{"front":"file_identity_document_success"}}}})]:
    s,o,_=sp.call("POST","/v1/accounts/"+i,d)
    if s!=200: sp.show(label+" ERROR",s,o)
    else: print(label,"accepted (200)")
time.sleep(15)
s,a,_=sp.call("GET","/v1/accounts/"+i); sp.show("state",s,sp.state(a))
print(a["requirements"]["alternatives"], a.get("future_requirements",{}).get("currently_due"))
