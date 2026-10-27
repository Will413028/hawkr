import { ACTIVE_ROLE_STORAGE_KEY } from "../auth/active-role";
import { STRIP_LOCALE_SCRIPT } from "../intl/paths";
import { INITIAL_LOADER_WATCHDOG_MS } from "./initial-loader-watchdog";
import {
	PRODUCT_DETAIL_ID_ATTRIBUTE,
	PUBLIC_INITIAL_CONTENT_RULES,
	ROUTE_TERMINAL_READY_SELECTOR,
} from "./initial-route-ready";

export const INITIAL_PUBLIC_CONTENT_READY_CLASS =
	"hk-initial-public-content-ready";
export const INITIAL_PUBLIC_CONTENT_READY_AT =
	"data-hk-initial-public-content-ready-at";
export const INITIAL_PUBLIC_CONTENT_ENABLED_CLASS =
	"hk-initial-public-content-enabled";

/**
 * Public SSR content can paint before React hydrates. This bootstrap only owns
 * the splash's VISUAL fade: initial-loader-active remains React's independent
 * document-settled barrier for analytics/toast DOM injection.
 *
 * Guest-only rollout: a saved role (every signed-in account with a storefront
 * role has one) keeps the original handoff, including seller-workspace
 * redirects. Storage failures fail closed. The hint only disables an
 * optimization; it never grants access or proves auth.
 * Query-driven search/filter/page state currently settles on the client, so
 * non-canonical URLs retain the original gate until they have matching markers.
 * No UA detection, data fetch, or image/font wait. The same marker rules
 * drive React. Hidden marker spans are intentional; hidden ANCESTORS are
 * Flight staging and must never release the splash. Scan once per frame, not
 * once per parser mutation. All observation ends at readiness or the watchdog.
 * Keep this literal self-contained: function.toString is not bundler-safe.
 */
export const INITIAL_PUBLIC_CONTENT_SCRIPT = `!function(){
var root=document.documentElement,path=window.location.pathname;
if(window.location.search)return;
try{if(window.localStorage.getItem(${JSON.stringify(ACTIVE_ROLE_STORAGE_KEY)})!==null)return}catch(e){return}
var strip=${STRIP_LOCALE_SCRIPT},normalized=strip(path.length>1?path.replace(/\\/+$/,""):path);
var rules=${JSON.stringify(PUBLIC_INITIAL_CONTENT_RULES)},rule,match;
for(var i=0;i<rules.length;i++){var found=new RegExp(rules[i].pattern).exec(normalized);if(found){rule=rules[i];match=found;break}}
if(!rule)return;
root.classList.add(${JSON.stringify(INITIAL_PUBLIC_CONTENT_ENABLED_CLASS)});
var readyClass=${JSON.stringify(INITIAL_PUBLIC_CONTENT_READY_CLASS)},readyAt=${JSON.stringify(INITIAL_PUBLIC_CONTENT_READY_AT)};
var productId=rule.product?match[1]:null;
if(productId){try{productId=decodeURIComponent(productId)}catch(e){}}
var frame=0,done=false,observer,deadline;
function stop(){done=true;if(observer)observer.disconnect();window.cancelAnimationFrame(frame);window.clearTimeout(deadline);document.removeEventListener("readystatechange",schedule);window.removeEventListener("pagehide",stop)}
function hasCommittedMarker(selector,checkProduct){
var nodes=document.querySelectorAll(selector);
for(var n=0;n<nodes.length;n++){var node=nodes[n];if(!node.isConnected||!node.parentElement||node.parentElement.closest("[hidden]"))continue;
if(checkProduct&&node.getAttribute(${JSON.stringify(PRODUCT_DETAIL_ID_ATTRIBUTE)})!==productId)continue;
return true}return false}
function check(){frame=0;if(done)return;
if(window.location.pathname!==path||!root.classList.contains("initial-loader-active")||root.classList.contains(readyClass)){stop();return}
if(document.readyState==="loading")return;
if(!hasCommittedMarker(${JSON.stringify(ROUTE_TERMINAL_READY_SELECTOR)},false)){
for(var s=0;s<rule.selectors.length;s++){if(!hasCommittedMarker(rule.selectors[s],!!rule.product))return}}
root.setAttribute(readyAt,String(performance.now()));root.classList.add(readyClass);stop()}
function schedule(){if(!done&&!frame)frame=window.requestAnimationFrame(check)}
observer=new MutationObserver(schedule);observer.observe(root,{childList:true,subtree:true,attributes:true,attributeFilter:["hidden","class"]});
document.addEventListener("readystatechange",schedule);window.addEventListener("pagehide",stop,{once:true});
deadline=window.setTimeout(stop,${INITIAL_LOADER_WATCHDOG_MS});schedule()
}();`;
