(()=>{var Rt=Object.create;var qe=Object.defineProperty;var Bt=Object.getOwnPropertyDescriptor;var Dt=Object.getOwnPropertyNames;var Et=Object.getPrototypeOf,Lt=Object.prototype.hasOwnProperty;var Ot=(e,t)=>()=>(t||e((t={exports:{}}).exports,t),t.exports);var Tt=(e,t,a,d)=>{if(t&&typeof t=="object"||typeof t=="function")for(let c of Dt(t))!Lt.call(e,c)&&c!==a&&qe(e,c,{get:()=>t[c],enumerable:!(d=Bt(t,c))||d.enumerable});return e};var Ft=(e,t,a)=>(a=e!=null?Rt(Et(e)):{},Tt(t||!e||!e.__esModule?qe(a,"default",{value:e,enumerable:!0}):a,e));var ft=Ot((Pa,ot)=>{function Xe(e){return e instanceof Map?e.clear=e.delete=e.set=function(){throw new Error("map is read-only")}:e instanceof Set&&(e.add=e.clear=e.delete=function(){throw new Error("set is read-only")}),Object.freeze(e),Object.getOwnPropertyNames(e).forEach(t=>{let a=e[t],d=typeof a;(d==="object"||d==="function")&&!Object.isFrozen(a)&&Xe(a)}),e}var K=class{constructor(t){t.data===void 0&&(t.data={}),this.data=t.data,this.isMatchIgnored=!1}ignoreMatch(){this.isMatchIgnored=!0}};function je(e){return e.replace(/&/g,"&amp;").replace(/</g,"&lt;").replace(/>/g,"&gt;").replace(/"/g,"&quot;").replace(/'/g,"&#x27;")}function O(e,...t){let a=Object.create(null);for(let d in e)a[d]=e[d];return t.forEach(function(d){for(let c in d)a[c]=d[c]}),a}var vt="</span>",Ie=e=>!!e.scope,Ut=(e,{prefix:t})=>{if(e.startsWith("language:"))return e.replace("language:","language-");if(e.includes(".")){let a=e.split(".");return[`${t}${a.shift()}`,...a.map((d,c)=>`${d}${"_".repeat(c+1)}`)].join(" ")}return`${t}${e}`},fe=class{constructor(t,a){this.buffer="",this.classPrefix=a.classPrefix,t.walk(this)}addText(t){this.buffer+=je(t)}openNode(t){if(!Ie(t))return;let a=Ut(t.scope,{prefix:this.classPrefix});this.span(a)}closeNode(t){Ie(t)&&(this.buffer+=vt)}value(){return this.buffer}span(t){this.buffer+=`<span class="${t}">`}},Ne=(e={})=>{let t={children:[]};return Object.assign(t,e),t},q=class{constructor(){this.rootNode=Ne(),this.stack=[this.rootNode]}get top(){return this.stack[this.stack.length-1]}get root(){return this.rootNode}add(t){this.top.children.push(t)}openNode(t){let a=Ne({scope:t});this.add(a),this.stack.push(a)}closeNode(){if(this.stack.length>1)return this.stack.pop()}closeAllNodes(){for(;this.closeNode(););}toJSON(){return JSON.stringify(this.rootNode,null,4)}walk(t){return this.constructor._walk(t,this.rootNode)}static _walk(t,a){return typeof a=="string"?t.addText(a):a.children&&(t.openNode(a),a.children.forEach(d=>this._walk(t,d)),t.closeNode(a)),t}static _collapse(t){typeof t!="string"&&t.children&&(t.children.every(a=>typeof a=="string")?t.children=[t.children.join("")]:t.children.forEach(a=>{q._collapse(a)}))}},le=class extends q{constructor(t){super(),this.options=t}addText(t){t!==""&&this.add(t)}startScope(t){this.openNode(t)}endScope(){this.closeNode()}__addSublanguage(t,a){let d=t.root;a&&(d.scope=`language:${a}`),this.add(d)}toHTML(){return new fe(this,this.options).value()}finalize(){return this.closeAllNodes(),!0}};function I(e){return e?typeof e=="string"?e:e.source:null}function Ke(e){return v("(?=",e,")")}function Ht(e){return v("(?:",e,")*")}function _t(e){return v("(?:",e,")?")}function v(...e){return e.map(a=>I(a)).join("")}function qt(e){let t=e[e.length-1];return typeof t=="object"&&t.constructor===Object?(e.splice(e.length-1,1),t):{}}function ue(...e){return"("+(qt(e).capture?"":"?:")+e.map(d=>I(d)).join("|")+")"}function $e(e){return new RegExp(e.toString()+"|").exec("").length-1}function It(e,t){let a=e&&e.exec(t);return a&&a.index===0}var Nt=/\[(?:[^\\\]]|\\.)*\]|\(\??|\\([1-9][0-9]*)|\\./;function de(e,{joinWith:t}){let a=0;return e.map(d=>{a+=1;let c=a,S=I(d),l="";for(;S.length>0;){let f=Nt.exec(S);if(!f){l+=S;break}l+=S.substring(0,f.index),S=S.substring(f.index+f[0].length),f[0][0]==="\\"&&f[1]?l+="\\"+String(Number(f[1])+c):(l+=f[0],f[0]==="("&&a++)}return l}).map(d=>`(${d})`).join(t)}var Gt=/\b\B/,Ze="[a-zA-Z]\\w*",pe="[a-zA-Z_]\\w*",Je="\\b\\d+(\\.\\d+)?",Qe="(-?)(\\b0[xX][a-fA-F0-9]+|(\\b\\d+(\\.\\d*)?|\\.\\d+)([eE][-+]?\\d+)?)",Ye="\\b(0b[01]+)",Wt="!|!=|!==|%|%=|&|&&|&=|\\*|\\*=|\\+|\\+=|,|-|-=|/=|/|:|;|<<|<<=|<=|<|===|==|=|>>>=|>>=|>=|>>>|>>|>|\\?|\\[|\\{|\\(|\\^|\\^=|\\||\\|=|\\|\\||~",Vt=(e={})=>{let t=/^#![ ]*\//;return e.binary&&(e.begin=v(t,/.*\b/,e.binary,/\b.*/)),O({scope:"meta",begin:t,end:/$/,relevance:0,"on:begin":(a,d)=>{a.index!==0&&d.ignoreMatch()}},e)},N={begin:"\\\\[\\s\\S]",relevance:0},zt={scope:"string",begin:"'",end:"'",illegal:"\\n",contains:[N]},Xt={scope:"string",begin:'"',end:'"',illegal:"\\n",contains:[N]},jt={begin:/\b(a|an|the|are|I'm|isn't|don't|doesn't|won't|but|just|should|pretty|simply|enough|gonna|going|wtf|so|such|will|you|your|they|like|more)\b/},Z=function(e,t,a={}){let d=O({scope:"comment",begin:e,end:t,contains:[]},a);d.contains.push({scope:"doctag",begin:"[ ]*(?=(TODO|FIXME|NOTE|BUG|OPTIMIZE|HACK|XXX):)",end:/(TODO|FIXME|NOTE|BUG|OPTIMIZE|HACK|XXX):/,excludeBegin:!0,relevance:0});let c=ue("I","a","is","so","us","to","at","if","in","it","on",/[A-Za-z]+['](d|ve|re|ll|t|s|n)/,/[A-Za-z]+[-][a-z]+/,/[A-Za-z][a-z]{2,}/);return d.contains.push({begin:v(/[ ]+/,"(",c,/[.]?[:]?([.][ ]|[ ])/,"){3}")}),d},Kt=Z("//","$"),$t=Z("/\\*","\\*/"),Zt=Z("#","$"),Jt={scope:"number",begin:Je,relevance:0},Qt={scope:"number",begin:Qe,relevance:0},Yt={scope:"number",begin:Ye,relevance:0},ea={scope:"regexp",begin:/\/(?=[^/\n]*\/)/,end:/\/[gimuy]*/,contains:[N,{begin:/\[/,end:/\]/,relevance:0,contains:[N]}]},ta={scope:"title",begin:Ze,relevance:0},aa={scope:"title",begin:pe,relevance:0},ra={begin:"\\.\\s*"+pe,relevance:0},oa=function(e){return Object.assign(e,{"on:begin":(t,a)=>{a.data._beginMatch=t[1]},"on:end":(t,a)=>{a.data._beginMatch!==t[1]&&a.ignoreMatch()}})},j=Object.freeze({__proto__:null,APOS_STRING_MODE:zt,BACKSLASH_ESCAPE:N,BINARY_NUMBER_MODE:Yt,BINARY_NUMBER_RE:Ye,COMMENT:Z,C_BLOCK_COMMENT_MODE:$t,C_LINE_COMMENT_MODE:Kt,C_NUMBER_MODE:Qt,C_NUMBER_RE:Qe,END_SAME_AS_BEGIN:oa,HASH_COMMENT_MODE:Zt,IDENT_RE:Ze,MATCH_NOTHING_RE:Gt,METHOD_GUARD:ra,NUMBER_MODE:Jt,NUMBER_RE:Je,PHRASAL_WORDS_MODE:jt,QUOTE_STRING_MODE:Xt,REGEXP_MODE:ea,RE_STARTERS_RE:Wt,SHEBANG:Vt,TITLE_MODE:ta,UNDERSCORE_IDENT_RE:pe,UNDERSCORE_TITLE_MODE:aa});function fa(e,t){e.input[e.index-1]==="."&&t.ignoreMatch()}function la(e,t){e.className!==void 0&&(e.scope=e.className,delete e.className)}function sa(e,t){t&&e.beginKeywords&&(e.begin="\\b("+e.beginKeywords.split(" ").join("|")+")(?!\\.)(?=\\b|\\s)",e.__beforeBegin=fa,e.keywords=e.keywords||e.beginKeywords,delete e.beginKeywords,e.relevance===void 0&&(e.relevance=0))}function ua(e,t){Array.isArray(e.illegal)&&(e.illegal=ue(...e.illegal))}function da(e,t){if(e.match){if(e.begin||e.end)throw new Error("begin & end are not supported with match");e.begin=e.match,delete e.match}}function pa(e,t){e.relevance===void 0&&(e.relevance=1)}var na=(e,t)=>{if(!e.beforeMatch)return;if(e.starts)throw new Error("beforeMatch cannot be used with starts");let a=Object.assign({},e);Object.keys(e).forEach(d=>{delete e[d]}),e.keywords=a.keywords,e.begin=v(a.beforeMatch,Ke(a.begin)),e.starts={relevance:0,contains:[Object.assign(a,{endsParent:!0})]},e.relevance=0,delete a.beforeMatch},ia=["of","and","for","in","not","or","if","then","parent","list","value"],ma="keyword";function et(e,t,a=ma){let d=Object.create(null);return typeof e=="string"?c(a,e.split(" ")):Array.isArray(e)?c(a,e):Object.keys(e).forEach(function(S){Object.assign(d,et(e[S],t,S))}),d;function c(S,l){t&&(l=l.map(f=>f.toLowerCase())),l.forEach(function(f){let p=f.split("|");d[p[0]]=[S,xa(p[0],p[1])]})}}function xa(e,t){return t?Number(t):ca(e)?0:1}function ca(e){return ia.includes(e.toLowerCase())}var Ge={},F=e=>{console.error(e)},We=(e,...t)=>{console.log(`WARN: ${e}`,...t)},U=(e,t)=>{Ge[`${e}/${t}`]||(console.log(`Deprecated as of ${e}. ${t}`),Ge[`${e}/${t}`]=!0)},$=new Error;function tt(e,t,{key:a}){let d=0,c=e[a],S={},l={};for(let f=1;f<=t.length;f++)l[f+d]=c[f],S[f+d]=!0,d+=$e(t[f-1]);e[a]=l,e[a]._emit=S,e[a]._multi=!0}function ha(e){if(Array.isArray(e.begin)){if(e.skip||e.excludeBegin||e.returnBegin)throw F("skip, excludeBegin, returnBegin not compatible with beginScope: {}"),$;if(typeof e.beginScope!="object"||e.beginScope===null)throw F("beginScope must be object"),$;tt(e,e.begin,{key:"beginScope"}),e.begin=de(e.begin,{joinWith:""})}}function ga(e){if(Array.isArray(e.end)){if(e.skip||e.excludeEnd||e.returnEnd)throw F("skip, excludeEnd, returnEnd not compatible with endScope: {}"),$;if(typeof e.endScope!="object"||e.endScope===null)throw F("endScope must be object"),$;tt(e,e.end,{key:"endScope"}),e.end=de(e.end,{joinWith:""})}}function Ca(e){e.scope&&typeof e.scope=="object"&&e.scope!==null&&(e.beginScope=e.scope,delete e.scope)}function Sa(e){Ca(e),typeof e.beginScope=="string"&&(e.beginScope={_wrap:e.beginScope}),typeof e.endScope=="string"&&(e.endScope={_wrap:e.endScope}),ha(e),ga(e)}function wa(e){function t(l,f){return new RegExp(I(l),"m"+(e.case_insensitive?"i":"")+(e.unicodeRegex?"u":"")+(f?"g":""))}class a{constructor(){this.matchIndexes={},this.regexes=[],this.matchAt=1,this.position=0}addRule(f,p){p.position=this.position++,this.matchIndexes[this.matchAt]=p,this.regexes.push([p,f]),this.matchAt+=$e(f)+1}compile(){this.regexes.length===0&&(this.exec=()=>null);let f=this.regexes.map(p=>p[1]);this.matcherRe=t(de(f,{joinWith:"|"}),!0),this.lastIndex=0}exec(f){this.matcherRe.lastIndex=this.lastIndex;let p=this.matcherRe.exec(f);if(!p)return null;let b=p.findIndex((_,Q)=>Q>0&&_!==void 0),w=this.matchIndexes[b];return p.splice(0,b),Object.assign(p,w)}}class d{constructor(){this.rules=[],this.multiRegexes=[],this.count=0,this.lastIndex=0,this.regexIndex=0}getMatcher(f){if(this.multiRegexes[f])return this.multiRegexes[f];let p=new a;return this.rules.slice(f).forEach(([b,w])=>p.addRule(b,w)),p.compile(),this.multiRegexes[f]=p,p}resumingScanAtSamePosition(){return this.regexIndex!==0}considerAll(){this.regexIndex=0}addRule(f,p){this.rules.push([f,p]),p.type==="begin"&&this.count++}exec(f){let p=this.getMatcher(this.regexIndex);p.lastIndex=this.lastIndex;let b=p.exec(f);if(this.resumingScanAtSamePosition()&&!(b&&b.index===this.lastIndex)){let w=this.getMatcher(0);w.lastIndex=this.lastIndex+1,b=w.exec(f)}return b&&(this.regexIndex+=b.position+1,this.regexIndex===this.count&&this.considerAll()),b}}function c(l){let f=new d;return l.contains.forEach(p=>f.addRule(p.begin,{rule:p,type:"begin"})),l.terminatorEnd&&f.addRule(l.terminatorEnd,{type:"end"}),l.illegal&&f.addRule(l.illegal,{type:"illegal"}),f}function S(l,f){let p=l;if(l.isCompiled)return p;[la,da,Sa,na].forEach(w=>w(l,f)),e.compilerExtensions.forEach(w=>w(l,f)),l.__beforeBegin=null,[sa,ua,pa].forEach(w=>w(l,f)),l.isCompiled=!0;let b=null;return typeof l.keywords=="object"&&l.keywords.$pattern&&(l.keywords=Object.assign({},l.keywords),b=l.keywords.$pattern,delete l.keywords.$pattern),b=b||/\w+/,l.keywords&&(l.keywords=et(l.keywords,e.case_insensitive)),p.keywordPatternRe=t(b,!0),f&&(l.begin||(l.begin=/\B|\b/),p.beginRe=t(p.begin),!l.end&&!l.endsWithParent&&(l.end=/\B|\b/),l.end&&(p.endRe=t(p.end)),p.terminatorEnd=I(p.end)||"",l.endsWithParent&&f.terminatorEnd&&(p.terminatorEnd+=(l.end?"|":"")+f.terminatorEnd)),l.illegal&&(p.illegalRe=t(l.illegal)),l.contains||(l.contains=[]),l.contains=[].concat(...l.contains.map(function(w){return Ma(w==="self"?l:w)})),l.contains.forEach(function(w){S(w,p)}),l.starts&&S(l.starts,f),p.matcher=c(p),p}if(e.compilerExtensions||(e.compilerExtensions=[]),e.contains&&e.contains.includes("self"))throw new Error("ERR: contains `self` is not supported at the top-level of a language.  See documentation.");return e.classNameAliases=O(e.classNameAliases||{}),S(e)}function at(e){return e?e.endsWithParent||at(e.starts):!1}function Ma(e){return e.variants&&!e.cachedVariants&&(e.cachedVariants=e.variants.map(function(t){return O(e,{variants:null},t)})),e.cachedVariants?e.cachedVariants:at(e)?O(e,{starts:e.starts?O(e.starts):null}):Object.isFrozen(e)?O(e):e}var ba="11.11.1",se=class extends Error{constructor(t,a){super(t),this.name="HTMLInjectionError",this.html=a}},oe=je,Ve=O,ze=Symbol("nomatch"),Aa=7,rt=function(e){let t=Object.create(null),a=Object.create(null),d=[],c=!0,S="Could not find the language '{}', did you forget to load/include a language module?",l={disableAutodetect:!0,name:"Plain text",contains:[]},f={ignoreUnescapedHTML:!1,throwUnescapedHTML:!1,noHighlightRe:/^(no-?highlight)$/i,languageDetectRe:/\blang(?:uage)?-([\w-]+)\b/i,classPrefix:"hljs-",cssSelector:"pre code",languages:null,__emitter:le};function p(r){return f.noHighlightRe.test(r)}function b(r){let u=r.className+" ";u+=r.parentNode?r.parentNode.className:"";let x=f.languageDetectRe.exec(u);if(x){let g=E(x[1]);return g||(We(S.replace("{}",x[1])),We("Falling back to no-highlight mode for this block.",r)),g?x[1]:"no-highlight"}return u.split(/\s+/).find(g=>p(g)||E(g))}function w(r,u,x){let g="",M="";typeof u=="object"?(g=r,x=u.ignoreIllegals,M=u.language):(U("10.7.0","highlight(lang, code, ...args) has been deprecated."),U("10.7.0",`Please use highlight(code, options) instead.
https://github.com/highlightjs/highlight.js/issues/2277`),M=r,g=u),x===void 0&&(x=!0);let y={code:g,language:M};W("before:highlight",y);let L=y.result?y.result:_(y.language,y.code,x);return L.code=y.code,W("after:highlight",L),L}function _(r,u,x,g){let M=Object.create(null);function y(o,s){return o.keywords[s]}function L(){if(!n.keywords){A.addText(C);return}let o=0;n.keywordPatternRe.lastIndex=0;let s=n.keywordPatternRe.exec(C),m="";for(;s;){m+=C.substring(o,s.index);let h=B.case_insensitive?s[0].toLowerCase():s[0],k=y(n,h);if(k){let[D,Pt]=k;if(A.addText(m),m="",M[h]=(M[h]||0)+1,M[h]<=Aa&&(X+=Pt),D.startsWith("_"))m+=s[0];else{let yt=B.classNameAliases[D]||D;R(s[0],yt)}}else m+=s[0];o=n.keywordPatternRe.lastIndex,s=n.keywordPatternRe.exec(C)}m+=C.substring(o),A.addText(m)}function V(){if(C==="")return;let o=null;if(typeof n.subLanguage=="string"){if(!t[n.subLanguage]){A.addText(C);return}o=_(n.subLanguage,C,!0,_e[n.subLanguage]),_e[n.subLanguage]=o._top}else o=Y(C,n.subLanguage.length?n.subLanguage:null);n.relevance>0&&(X+=o.relevance),A.__addSublanguage(o._emitter,o.language)}function P(){n.subLanguage!=null?V():L(),C=""}function R(o,s){o!==""&&(A.startScope(s),A.addText(o),A.endScope())}function Fe(o,s){let m=1,h=s.length-1;for(;m<=h;){if(!o._emit[m]){m++;continue}let k=B.classNameAliases[o[m]]||o[m],D=s[m];k?R(D,k):(C=D,L(),C=""),m++}}function ve(o,s){return o.scope&&typeof o.scope=="string"&&A.openNode(B.classNameAliases[o.scope]||o.scope),o.beginScope&&(o.beginScope._wrap?(R(C,B.classNameAliases[o.beginScope._wrap]||o.beginScope._wrap),C=""):o.beginScope._multi&&(Fe(o.beginScope,s),C="")),n=Object.create(o,{parent:{value:n}}),n}function Ue(o,s,m){let h=It(o.endRe,m);if(h){if(o["on:end"]){let k=new K(o);o["on:end"](s,k),k.isMatchIgnored&&(h=!1)}if(h){for(;o.endsParent&&o.parent;)o=o.parent;return o}}if(o.endsWithParent)return Ue(o.parent,s,m)}function wt(o){return n.matcher.regexIndex===0?(C+=o[0],1):(re=!0,0)}function Mt(o){let s=o[0],m=o.rule,h=new K(m),k=[m.__beforeBegin,m["on:begin"]];for(let D of k)if(D&&(D(o,h),h.isMatchIgnored))return wt(s);return m.skip?C+=s:(m.excludeBegin&&(C+=s),P(),!m.returnBegin&&!m.excludeBegin&&(C=s)),ve(m,o),m.returnBegin?0:s.length}function bt(o){let s=o[0],m=u.substring(o.index),h=Ue(n,o,m);if(!h)return ze;let k=n;n.endScope&&n.endScope._wrap?(P(),R(s,n.endScope._wrap)):n.endScope&&n.endScope._multi?(P(),Fe(n.endScope,o)):k.skip?C+=s:(k.returnEnd||k.excludeEnd||(C+=s),P(),k.excludeEnd&&(C=s));do n.scope&&A.closeNode(),!n.skip&&!n.subLanguage&&(X+=n.relevance),n=n.parent;while(n!==h.parent);return h.starts&&ve(h.starts,o),k.returnEnd?0:s.length}function At(){let o=[];for(let s=n;s!==B;s=s.parent)s.scope&&o.unshift(s.scope);o.forEach(s=>A.openNode(s))}let z={};function He(o,s){let m=s&&s[0];if(C+=o,m==null)return P(),0;if(z.type==="begin"&&s.type==="end"&&z.index===s.index&&m===""){if(C+=u.slice(s.index,s.index+1),!c){let h=new Error(`0 width match regex (${r})`);throw h.languageName=r,h.badRule=z.rule,h}return 1}if(z=s,s.type==="begin")return Mt(s);if(s.type==="illegal"&&!x){let h=new Error('Illegal lexeme "'+m+'" for mode "'+(n.scope||"<unnamed>")+'"');throw h.mode=n,h}else if(s.type==="end"){let h=bt(s);if(h!==ze)return h}if(s.type==="illegal"&&m==="")return C+=`
`,1;if(ae>1e5&&ae>s.index*3)throw new Error("potential infinite loop, way more iterations than matches");return C+=m,m.length}let B=E(r);if(!B)throw F(S.replace("{}",r)),new Error('Unknown language: "'+r+'"');let kt=wa(B),te="",n=g||kt,_e={},A=new f.__emitter(f);At();let C="",X=0,T=0,ae=0,re=!1;try{if(B.__emitTokens)B.__emitTokens(u,A);else{for(n.matcher.considerAll();;){ae++,re?re=!1:n.matcher.considerAll(),n.matcher.lastIndex=T;let o=n.matcher.exec(u);if(!o)break;let s=u.substring(T,o.index),m=He(s,o);T=o.index+m}He(u.substring(T))}return A.finalize(),te=A.toHTML(),{language:r,value:te,relevance:X,illegal:!1,_emitter:A,_top:n}}catch(o){if(o.message&&o.message.includes("Illegal"))return{language:r,value:oe(u),illegal:!0,relevance:0,_illegalBy:{message:o.message,index:T,context:u.slice(T-100,T+100),mode:o.mode,resultSoFar:te},_emitter:A};if(c)return{language:r,value:oe(u),illegal:!1,relevance:0,errorRaised:o,_emitter:A,_top:n};throw o}}function Q(r){let u={value:oe(r),illegal:!1,relevance:0,_top:l,_emitter:new f.__emitter(f)};return u._emitter.addText(r),u}function Y(r,u){u=u||f.languages||Object.keys(t);let x=Q(r),g=u.filter(E).filter(Te).map(P=>_(P,r,!1));g.unshift(x);let M=g.sort((P,R)=>{if(P.relevance!==R.relevance)return R.relevance-P.relevance;if(P.language&&R.language){if(E(P.language).supersetOf===R.language)return 1;if(E(R.language).supersetOf===P.language)return-1}return 0}),[y,L]=M,V=y;return V.secondBest=L,V}function dt(r,u,x){let g=u&&a[u]||x;r.classList.add("hljs"),r.classList.add(`language-${g}`)}function ee(r){let u=null,x=b(r);if(p(x))return;if(W("before:highlightElement",{el:r,language:x}),r.dataset.highlighted){console.log("Element previously highlighted. To highlight again, first unset `dataset.highlighted`.",r);return}if(r.children.length>0&&(f.ignoreUnescapedHTML||(console.warn("One of your code blocks includes unescaped HTML. This is a potentially serious security risk."),console.warn("https://github.com/highlightjs/highlight.js/wiki/security"),console.warn("The element with unescaped HTML:"),console.warn(r)),f.throwUnescapedHTML))throw new se("One of your code blocks includes unescaped HTML.",r.innerHTML);u=r;let g=u.textContent,M=x?w(g,{language:x,ignoreIllegals:!0}):Y(g);r.innerHTML=M.value,r.dataset.highlighted="yes",dt(r,x,M.language),r.result={language:M.language,re:M.relevance,relevance:M.relevance},M.secondBest&&(r.secondBest={language:M.secondBest.language,relevance:M.secondBest.relevance}),W("after:highlightElement",{el:r,result:M,text:g})}function pt(r){f=Ve(f,r)}let nt=()=>{G(),U("10.6.0","initHighlighting() deprecated.  Use highlightAll() now.")};function it(){G(),U("10.6.0","initHighlightingOnLoad() deprecated.  Use highlightAll() now.")}let Le=!1;function G(){function r(){G()}if(document.readyState==="loading"){Le||window.addEventListener("DOMContentLoaded",r,!1),Le=!0;return}document.querySelectorAll(f.cssSelector).forEach(ee)}function mt(r,u){let x=null;try{x=u(e)}catch(g){if(F("Language definition for '{}' could not be registered.".replace("{}",r)),c)F(g);else throw g;x=l}x.name||(x.name=r),t[r]=x,x.rawDefinition=u.bind(null,e),x.aliases&&Oe(x.aliases,{languageName:r})}function xt(r){delete t[r];for(let u of Object.keys(a))a[u]===r&&delete a[u]}function ct(){return Object.keys(t)}function E(r){return r=(r||"").toLowerCase(),t[r]||t[a[r]]}function Oe(r,{languageName:u}){typeof r=="string"&&(r=[r]),r.forEach(x=>{a[x.toLowerCase()]=u})}function Te(r){let u=E(r);return u&&!u.disableAutodetect}function ht(r){r["before:highlightBlock"]&&!r["before:highlightElement"]&&(r["before:highlightElement"]=u=>{r["before:highlightBlock"](Object.assign({block:u.el},u))}),r["after:highlightBlock"]&&!r["after:highlightElement"]&&(r["after:highlightElement"]=u=>{r["after:highlightBlock"](Object.assign({block:u.el},u))})}function gt(r){ht(r),d.push(r)}function Ct(r){let u=d.indexOf(r);u!==-1&&d.splice(u,1)}function W(r,u){let x=r;d.forEach(function(g){g[x]&&g[x](u)})}function St(r){return U("10.7.0","highlightBlock will be removed entirely in v12.0"),U("10.7.0","Please use highlightElement now."),ee(r)}Object.assign(e,{highlight:w,highlightAuto:Y,highlightAll:G,highlightElement:ee,highlightBlock:St,configure:pt,initHighlighting:nt,initHighlightingOnLoad:it,registerLanguage:mt,unregisterLanguage:xt,listLanguages:ct,getLanguage:E,registerAliases:Oe,autoDetection:Te,inherit:Ve,addPlugin:gt,removePlugin:Ct}),e.debugMode=function(){c=!1},e.safeMode=function(){c=!0},e.versionString=ba,e.regex={concat:v,lookahead:Ke,either:ue,optional:_t,anyNumberOfTimes:Ht};for(let r in j)typeof j[r]=="object"&&Xe(j[r]);return Object.assign(e,j),e},H=rt({});H.newInstance=()=>rt({});ot.exports=H;H.HighlightJS=H;H.default=H});var lt=Ft(ft(),1);var st=lt.default;var ut=(e,t,a=[])=>{let d=document.createElementNS("http://www.w3.org/2000/svg",e);return Object.keys(t).forEach(c=>{d.setAttribute(c,String(t[c]))}),a.length&&a.forEach(c=>{let S=ut(...c);d.appendChild(S)}),d},ne=([e,t,a])=>ut(e,t,a);var i={xmlns:"http://www.w3.org/2000/svg",width:24,height:24,viewBox:"0 0 24 24",fill:"none",stroke:"currentColor","stroke-width":2,"stroke-linecap":"round","stroke-linejoin":"round"};var ie=["svg",i,[["path",{d:"m12 19-7-7 7-7"}],["path",{d:"M19 12H5"}]]];var me=["svg",i,[["path",{d:"M5 12h14"}],["path",{d:"m12 5 7 7-7 7"}]]];var xe=["svg",i,[["path",{d:"M20 6 9 17l-5-5"}]]];var ce=["svg",i,[["path",{d:"M15 2H9a1 1 0 0 0-1 1v2c0 .6.4 1 1 1h6c.6 0 1-.4 1-1V3c0-.6-.4-1-1-1Z"}],["path",{d:"M8 4H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2M16 4h2a2 2 0 0 1 2 2v2M11 14h10"}],["path",{d:"m17 10 4 4-4 4"}]]];var he=["svg",i,[["rect",{width:"14",height:"14",x:"8",y:"8",rx:"2",ry:"2"}],["path",{d:"M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"}]]];var ge=["svg",i,[["path",{d:"M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"}],["polyline",{points:"7 10 12 15 17 10"}],["line",{x1:"12",x2:"12",y1:"15",y2:"3"}]]];var Ce=["svg",i,[["path",{d:"M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"}],["path",{d:"M14 2v4a2 2 0 0 0 2 2h4"}],["path",{d:"M10 9H8"}],["path",{d:"M16 13H8"}],["path",{d:"M16 17H8"}]]];var Se=["svg",i,[["path",{d:"m6 14 1.5-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.54 6a2 2 0 0 1-1.95 1.5H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H18a2 2 0 0 1 2 2v2"}]]];var we=["svg",i,[["path",{d:"M8 3H5a2 2 0 0 0-2 2v3"}],["path",{d:"M21 8V5a2 2 0 0 0-2-2h-3"}],["path",{d:"M3 16v3a2 2 0 0 0 2 2h3"}],["path",{d:"M16 21h3a2 2 0 0 0 2-2v-3"}]]];var Me=["svg",i,[["path",{d:"M8 3v3a2 2 0 0 1-2 2H3"}],["path",{d:"M21 8h-3a2 2 0 0 1-2-2V3"}],["path",{d:"M3 16h3a2 2 0 0 1 2 2v3"}],["path",{d:"M16 21v-3a2 2 0 0 1 2-2h3"}]]];var J=["svg",i,[["rect",{width:"18",height:"18",x:"3",y:"3",rx:"2"}],["path",{d:"M3 9h18"}],["path",{d:"M9 21V9"}]]];var be=["svg",i,[["polygon",{points:"6 3 20 12 6 21 6 3"}]]];var Ae=["svg",i,[["path",{d:"M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"}],["path",{d:"M3 3v5h5"}]]];var ke=["svg",i,[["circle",{cx:"11",cy:"11",r:"8"}],["path",{d:"m21 21-4.3-4.3"}]]];var Pe=["svg",i,[["rect",{width:"20",height:"8",x:"2",y:"2",rx:"2",ry:"2"}],["rect",{width:"20",height:"8",x:"2",y:"14",rx:"2",ry:"2"}],["line",{x1:"6",x2:"6.01",y1:"6",y2:"6"}],["line",{x1:"6",x2:"6.01",y1:"18",y2:"18"}]]];var ye=["svg",i,[["path",{d:"M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"}],["circle",{cx:"12",cy:"12",r:"3"}]]];var Re=["svg",i,[["polyline",{points:"4 17 10 11 4 5"}],["line",{x1:"12",x2:"20",y1:"19",y2:"19"}]]];var Be=["svg",i,[["path",{d:"M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"}],["polyline",{points:"17 8 12 3 7 8"}],["line",{x1:"12",x2:"12",y1:"3",y2:"15"}]]];var De=["svg",i,[["line",{x1:"3",x2:"21",y1:"6",y2:"6"}],["path",{d:"M3 12h15a3 3 0 1 1 0 6h-4"}],["polyline",{points:"16 16 14 18 16 20"}],["line",{x1:"3",x2:"10",y1:"18",y2:"18"}]]];var Ee=["svg",i,[["path",{d:"M18 6 6 18"}],["path",{d:"m6 6 12 12"}]]];window.LogHighlight=st;window.LogIcons={createElement:ne,FileText:Ce,FolderOpen:Se,Upload:Be,Server:Pe,ClipboardPaste:ce,Settings:ye,Download:ge,Search:ke,RotateCcw:Ae,Copy:he,Maximize:we,Minimize:Me,ArrowLeft:ie,ArrowRight:me,Play:be,X:Ee,PanelsTopLeft:J,Check:xe,Terminal:Re,WrapText:De};})();
/*! Bundled license information:

lucide/dist/esm/createElement.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/defaultAttributes.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/arrow-left.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/arrow-right.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/check.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/clipboard-paste.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/copy.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/download.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/file-text.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/folder-open.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/maximize.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/minimize.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/panels-top-left.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/play.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/rotate-ccw.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/search.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/server.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/settings.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/terminal.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/upload.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/wrap-text.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/x.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/lucide.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)
*/
