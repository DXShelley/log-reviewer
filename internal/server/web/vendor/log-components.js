(()=>{var Et=Object.create;var Ne=Object.defineProperty;var Dt=Object.getOwnPropertyDescriptor;var Ot=Object.getOwnPropertyNames;var Lt=Object.getPrototypeOf,Tt=Object.prototype.hasOwnProperty;var Ft=(e,t)=>()=>(t||e((t={exports:{}}).exports,t),t.exports);var vt=(e,t,a,d)=>{if(t&&typeof t=="object"||typeof t=="function")for(let c of Ot(t))!Tt.call(e,c)&&c!==a&&Ne(e,c,{get:()=>t[c],enumerable:!(d=Dt(t,c))||d.enumerable});return e};var Ut=(e,t,a)=>(a=e!=null?Et(Lt(e)):{},vt(t||!e||!e.__esModule?Ne(a,"default",{value:e,enumerable:!0}):a,e));var st=Ft((Ra,lt)=>{function Ke(e){return e instanceof Map?e.clear=e.delete=e.set=function(){throw new Error("map is read-only")}:e instanceof Set&&(e.add=e.clear=e.delete=function(){throw new Error("set is read-only")}),Object.freeze(e),Object.getOwnPropertyNames(e).forEach(t=>{let a=e[t],d=typeof a;(d==="object"||d==="function")&&!Object.isFrozen(a)&&Ke(a)}),e}var K=class{constructor(t){t.data===void 0&&(t.data={}),this.data=t.data,this.isMatchIgnored=!1}ignoreMatch(){this.isMatchIgnored=!0}};function $e(e){return e.replace(/&/g,"&amp;").replace(/</g,"&lt;").replace(/>/g,"&gt;").replace(/"/g,"&quot;").replace(/'/g,"&#x27;")}function L(e,...t){let a=Object.create(null);for(let d in e)a[d]=e[d];return t.forEach(function(d){for(let c in d)a[c]=d[c]}),a}var Ht="</span>",Ge=e=>!!e.scope,_t=(e,{prefix:t})=>{if(e.startsWith("language:"))return e.replace("language:","language-");if(e.includes(".")){let a=e.split(".");return[`${t}${a.shift()}`,...a.map((d,c)=>`${d}${"_".repeat(c+1)}`)].join(" ")}return`${t}${e}`},fe=class{constructor(t,a){this.buffer="",this.classPrefix=a.classPrefix,t.walk(this)}addText(t){this.buffer+=$e(t)}openNode(t){if(!Ge(t))return;let a=_t(t.scope,{prefix:this.classPrefix});this.span(a)}closeNode(t){Ge(t)&&(this.buffer+=Ht)}value(){return this.buffer}span(t){this.buffer+=`<span class="${t}">`}},We=(e={})=>{let t={children:[]};return Object.assign(t,e),t},q=class{constructor(){this.rootNode=We(),this.stack=[this.rootNode]}get top(){return this.stack[this.stack.length-1]}get root(){return this.rootNode}add(t){this.top.children.push(t)}openNode(t){let a=We({scope:t});this.add(a),this.stack.push(a)}closeNode(){if(this.stack.length>1)return this.stack.pop()}closeAllNodes(){for(;this.closeNode(););}toJSON(){return JSON.stringify(this.rootNode,null,4)}walk(t){return this.constructor._walk(t,this.rootNode)}static _walk(t,a){return typeof a=="string"?t.addText(a):a.children&&(t.openNode(a),a.children.forEach(d=>this._walk(t,d)),t.closeNode(a)),t}static _collapse(t){typeof t!="string"&&t.children&&(t.children.every(a=>typeof a=="string")?t.children=[t.children.join("")]:t.children.forEach(a=>{q._collapse(a)}))}},le=class extends q{constructor(t){super(),this.options=t}addText(t){t!==""&&this.add(t)}startScope(t){this.openNode(t)}endScope(){this.closeNode()}__addSublanguage(t,a){let d=t.root;a&&(d.scope=`language:${a}`),this.add(d)}toHTML(){return new fe(this,this.options).value()}finalize(){return this.closeAllNodes(),!0}};function I(e){return e?typeof e=="string"?e:e.source:null}function Ze(e){return v("(?=",e,")")}function qt(e){return v("(?:",e,")*")}function It(e){return v("(?:",e,")?")}function v(...e){return e.map(a=>I(a)).join("")}function Nt(e){let t=e[e.length-1];return typeof t=="object"&&t.constructor===Object?(e.splice(e.length-1,1),t):{}}function ue(...e){return"("+(Nt(e).capture?"":"?:")+e.map(d=>I(d)).join("|")+")"}function Je(e){return new RegExp(e.toString()+"|").exec("").length-1}function Gt(e,t){let a=e&&e.exec(t);return a&&a.index===0}var Wt=/\[(?:[^\\\]]|\\.)*\]|\(\??|\\([1-9][0-9]*)|\\./;function de(e,{joinWith:t}){let a=0;return e.map(d=>{a+=1;let c=a,S=I(d),l="";for(;S.length>0;){let f=Wt.exec(S);if(!f){l+=S;break}l+=S.substring(0,f.index),S=S.substring(f.index+f[0].length),f[0][0]==="\\"&&f[1]?l+="\\"+String(Number(f[1])+c):(l+=f[0],f[0]==="("&&a++)}return l}).map(d=>`(${d})`).join(t)}var Vt=/\b\B/,Qe="[a-zA-Z]\\w*",pe="[a-zA-Z_]\\w*",Ye="\\b\\d+(\\.\\d+)?",et="(-?)(\\b0[xX][a-fA-F0-9]+|(\\b\\d+(\\.\\d*)?|\\.\\d+)([eE][-+]?\\d+)?)",tt="\\b(0b[01]+)",zt="!|!=|!==|%|%=|&|&&|&=|\\*|\\*=|\\+|\\+=|,|-|-=|/=|/|:|;|<<|<<=|<=|<|===|==|=|>>>=|>>=|>=|>>>|>>|>|\\?|\\[|\\{|\\(|\\^|\\^=|\\||\\|=|\\|\\||~",Xt=(e={})=>{let t=/^#![ ]*\//;return e.binary&&(e.begin=v(t,/.*\b/,e.binary,/\b.*/)),L({scope:"meta",begin:t,end:/$/,relevance:0,"on:begin":(a,d)=>{a.index!==0&&d.ignoreMatch()}},e)},N={begin:"\\\\[\\s\\S]",relevance:0},jt={scope:"string",begin:"'",end:"'",illegal:"\\n",contains:[N]},Kt={scope:"string",begin:'"',end:'"',illegal:"\\n",contains:[N]},$t={begin:/\b(a|an|the|are|I'm|isn't|don't|doesn't|won't|but|just|should|pretty|simply|enough|gonna|going|wtf|so|such|will|you|your|they|like|more)\b/},Z=function(e,t,a={}){let d=L({scope:"comment",begin:e,end:t,contains:[]},a);d.contains.push({scope:"doctag",begin:"[ ]*(?=(TODO|FIXME|NOTE|BUG|OPTIMIZE|HACK|XXX):)",end:/(TODO|FIXME|NOTE|BUG|OPTIMIZE|HACK|XXX):/,excludeBegin:!0,relevance:0});let c=ue("I","a","is","so","us","to","at","if","in","it","on",/[A-Za-z]+['](d|ve|re|ll|t|s|n)/,/[A-Za-z]+[-][a-z]+/,/[A-Za-z][a-z]{2,}/);return d.contains.push({begin:v(/[ ]+/,"(",c,/[.]?[:]?([.][ ]|[ ])/,"){3}")}),d},Zt=Z("//","$"),Jt=Z("/\\*","\\*/"),Qt=Z("#","$"),Yt={scope:"number",begin:Ye,relevance:0},ea={scope:"number",begin:et,relevance:0},ta={scope:"number",begin:tt,relevance:0},aa={scope:"regexp",begin:/\/(?=[^/\n]*\/)/,end:/\/[gimuy]*/,contains:[N,{begin:/\[/,end:/\]/,relevance:0,contains:[N]}]},ra={scope:"title",begin:Qe,relevance:0},oa={scope:"title",begin:pe,relevance:0},fa={begin:"\\.\\s*"+pe,relevance:0},la=function(e){return Object.assign(e,{"on:begin":(t,a)=>{a.data._beginMatch=t[1]},"on:end":(t,a)=>{a.data._beginMatch!==t[1]&&a.ignoreMatch()}})},j=Object.freeze({__proto__:null,APOS_STRING_MODE:jt,BACKSLASH_ESCAPE:N,BINARY_NUMBER_MODE:ta,BINARY_NUMBER_RE:tt,COMMENT:Z,C_BLOCK_COMMENT_MODE:Jt,C_LINE_COMMENT_MODE:Zt,C_NUMBER_MODE:ea,C_NUMBER_RE:et,END_SAME_AS_BEGIN:la,HASH_COMMENT_MODE:Qt,IDENT_RE:Qe,MATCH_NOTHING_RE:Vt,METHOD_GUARD:fa,NUMBER_MODE:Yt,NUMBER_RE:Ye,PHRASAL_WORDS_MODE:$t,QUOTE_STRING_MODE:Kt,REGEXP_MODE:aa,RE_STARTERS_RE:zt,SHEBANG:Xt,TITLE_MODE:ra,UNDERSCORE_IDENT_RE:pe,UNDERSCORE_TITLE_MODE:oa});function sa(e,t){e.input[e.index-1]==="."&&t.ignoreMatch()}function ua(e,t){e.className!==void 0&&(e.scope=e.className,delete e.className)}function da(e,t){t&&e.beginKeywords&&(e.begin="\\b("+e.beginKeywords.split(" ").join("|")+")(?!\\.)(?=\\b|\\s)",e.__beforeBegin=sa,e.keywords=e.keywords||e.beginKeywords,delete e.beginKeywords,e.relevance===void 0&&(e.relevance=0))}function pa(e,t){Array.isArray(e.illegal)&&(e.illegal=ue(...e.illegal))}function na(e,t){if(e.match){if(e.begin||e.end)throw new Error("begin & end are not supported with match");e.begin=e.match,delete e.match}}function ia(e,t){e.relevance===void 0&&(e.relevance=1)}var ma=(e,t)=>{if(!e.beforeMatch)return;if(e.starts)throw new Error("beforeMatch cannot be used with starts");let a=Object.assign({},e);Object.keys(e).forEach(d=>{delete e[d]}),e.keywords=a.keywords,e.begin=v(a.beforeMatch,Ze(a.begin)),e.starts={relevance:0,contains:[Object.assign(a,{endsParent:!0})]},e.relevance=0,delete a.beforeMatch},xa=["of","and","for","in","not","or","if","then","parent","list","value"],ca="keyword";function at(e,t,a=ca){let d=Object.create(null);return typeof e=="string"?c(a,e.split(" ")):Array.isArray(e)?c(a,e):Object.keys(e).forEach(function(S){Object.assign(d,at(e[S],t,S))}),d;function c(S,l){t&&(l=l.map(f=>f.toLowerCase())),l.forEach(function(f){let p=f.split("|");d[p[0]]=[S,ha(p[0],p[1])]})}}function ha(e,t){return t?Number(t):ga(e)?0:1}function ga(e){return xa.includes(e.toLowerCase())}var Ve={},F=e=>{console.error(e)},ze=(e,...t)=>{console.log(`WARN: ${e}`,...t)},U=(e,t)=>{Ve[`${e}/${t}`]||(console.log(`Deprecated as of ${e}. ${t}`),Ve[`${e}/${t}`]=!0)},$=new Error;function rt(e,t,{key:a}){let d=0,c=e[a],S={},l={};for(let f=1;f<=t.length;f++)l[f+d]=c[f],S[f+d]=!0,d+=Je(t[f-1]);e[a]=l,e[a]._emit=S,e[a]._multi=!0}function Ca(e){if(Array.isArray(e.begin)){if(e.skip||e.excludeBegin||e.returnBegin)throw F("skip, excludeBegin, returnBegin not compatible with beginScope: {}"),$;if(typeof e.beginScope!="object"||e.beginScope===null)throw F("beginScope must be object"),$;rt(e,e.begin,{key:"beginScope"}),e.begin=de(e.begin,{joinWith:""})}}function Sa(e){if(Array.isArray(e.end)){if(e.skip||e.excludeEnd||e.returnEnd)throw F("skip, excludeEnd, returnEnd not compatible with endScope: {}"),$;if(typeof e.endScope!="object"||e.endScope===null)throw F("endScope must be object"),$;rt(e,e.end,{key:"endScope"}),e.end=de(e.end,{joinWith:""})}}function wa(e){e.scope&&typeof e.scope=="object"&&e.scope!==null&&(e.beginScope=e.scope,delete e.scope)}function Ma(e){wa(e),typeof e.beginScope=="string"&&(e.beginScope={_wrap:e.beginScope}),typeof e.endScope=="string"&&(e.endScope={_wrap:e.endScope}),Ca(e),Sa(e)}function ba(e){function t(l,f){return new RegExp(I(l),"m"+(e.case_insensitive?"i":"")+(e.unicodeRegex?"u":"")+(f?"g":""))}class a{constructor(){this.matchIndexes={},this.regexes=[],this.matchAt=1,this.position=0}addRule(f,p){p.position=this.position++,this.matchIndexes[this.matchAt]=p,this.regexes.push([p,f]),this.matchAt+=Je(f)+1}compile(){this.regexes.length===0&&(this.exec=()=>null);let f=this.regexes.map(p=>p[1]);this.matcherRe=t(de(f,{joinWith:"|"}),!0),this.lastIndex=0}exec(f){this.matcherRe.lastIndex=this.lastIndex;let p=this.matcherRe.exec(f);if(!p)return null;let b=p.findIndex((_,Q)=>Q>0&&_!==void 0),w=this.matchIndexes[b];return p.splice(0,b),Object.assign(p,w)}}class d{constructor(){this.rules=[],this.multiRegexes=[],this.count=0,this.lastIndex=0,this.regexIndex=0}getMatcher(f){if(this.multiRegexes[f])return this.multiRegexes[f];let p=new a;return this.rules.slice(f).forEach(([b,w])=>p.addRule(b,w)),p.compile(),this.multiRegexes[f]=p,p}resumingScanAtSamePosition(){return this.regexIndex!==0}considerAll(){this.regexIndex=0}addRule(f,p){this.rules.push([f,p]),p.type==="begin"&&this.count++}exec(f){let p=this.getMatcher(this.regexIndex);p.lastIndex=this.lastIndex;let b=p.exec(f);if(this.resumingScanAtSamePosition()&&!(b&&b.index===this.lastIndex)){let w=this.getMatcher(0);w.lastIndex=this.lastIndex+1,b=w.exec(f)}return b&&(this.regexIndex+=b.position+1,this.regexIndex===this.count&&this.considerAll()),b}}function c(l){let f=new d;return l.contains.forEach(p=>f.addRule(p.begin,{rule:p,type:"begin"})),l.terminatorEnd&&f.addRule(l.terminatorEnd,{type:"end"}),l.illegal&&f.addRule(l.illegal,{type:"illegal"}),f}function S(l,f){let p=l;if(l.isCompiled)return p;[ua,na,Ma,ma].forEach(w=>w(l,f)),e.compilerExtensions.forEach(w=>w(l,f)),l.__beforeBegin=null,[da,pa,ia].forEach(w=>w(l,f)),l.isCompiled=!0;let b=null;return typeof l.keywords=="object"&&l.keywords.$pattern&&(l.keywords=Object.assign({},l.keywords),b=l.keywords.$pattern,delete l.keywords.$pattern),b=b||/\w+/,l.keywords&&(l.keywords=at(l.keywords,e.case_insensitive)),p.keywordPatternRe=t(b,!0),f&&(l.begin||(l.begin=/\B|\b/),p.beginRe=t(p.begin),!l.end&&!l.endsWithParent&&(l.end=/\B|\b/),l.end&&(p.endRe=t(p.end)),p.terminatorEnd=I(p.end)||"",l.endsWithParent&&f.terminatorEnd&&(p.terminatorEnd+=(l.end?"|":"")+f.terminatorEnd)),l.illegal&&(p.illegalRe=t(l.illegal)),l.contains||(l.contains=[]),l.contains=[].concat(...l.contains.map(function(w){return Aa(w==="self"?l:w)})),l.contains.forEach(function(w){S(w,p)}),l.starts&&S(l.starts,f),p.matcher=c(p),p}if(e.compilerExtensions||(e.compilerExtensions=[]),e.contains&&e.contains.includes("self"))throw new Error("ERR: contains `self` is not supported at the top-level of a language.  See documentation.");return e.classNameAliases=L(e.classNameAliases||{}),S(e)}function ot(e){return e?e.endsWithParent||ot(e.starts):!1}function Aa(e){return e.variants&&!e.cachedVariants&&(e.cachedVariants=e.variants.map(function(t){return L(e,{variants:null},t)})),e.cachedVariants?e.cachedVariants:ot(e)?L(e,{starts:e.starts?L(e.starts):null}):Object.isFrozen(e)?L(e):e}var ka="11.11.1",se=class extends Error{constructor(t,a){super(t),this.name="HTMLInjectionError",this.html=a}},oe=$e,Xe=L,je=Symbol("nomatch"),Pa=7,ft=function(e){let t=Object.create(null),a=Object.create(null),d=[],c=!0,S="Could not find the language '{}', did you forget to load/include a language module?",l={disableAutodetect:!0,name:"Plain text",contains:[]},f={ignoreUnescapedHTML:!1,throwUnescapedHTML:!1,noHighlightRe:/^(no-?highlight)$/i,languageDetectRe:/\blang(?:uage)?-([\w-]+)\b/i,classPrefix:"hljs-",cssSelector:"pre code",languages:null,__emitter:le};function p(r){return f.noHighlightRe.test(r)}function b(r){let u=r.className+" ";u+=r.parentNode?r.parentNode.className:"";let x=f.languageDetectRe.exec(u);if(x){let g=D(x[1]);return g||(ze(S.replace("{}",x[1])),ze("Falling back to no-highlight mode for this block.",r)),g?x[1]:"no-highlight"}return u.split(/\s+/).find(g=>p(g)||D(g))}function w(r,u,x){let g="",M="";typeof u=="object"?(g=r,x=u.ignoreIllegals,M=u.language):(U("10.7.0","highlight(lang, code, ...args) has been deprecated."),U("10.7.0",`Please use highlight(code, options) instead.
https://github.com/highlightjs/highlight.js/issues/2277`),M=r,g=u),x===void 0&&(x=!0);let y={code:g,language:M};W("before:highlight",y);let O=y.result?y.result:_(y.language,y.code,x);return O.code=y.code,W("after:highlight",O),O}function _(r,u,x,g){let M=Object.create(null);function y(o,s){return o.keywords[s]}function O(){if(!i.keywords){A.addText(C);return}let o=0;i.keywordPatternRe.lastIndex=0;let s=i.keywordPatternRe.exec(C),m="";for(;s;){m+=C.substring(o,s.index);let h=B.case_insensitive?s[0].toLowerCase():s[0],k=y(i,h);if(k){let[E,Rt]=k;if(A.addText(m),m="",M[h]=(M[h]||0)+1,M[h]<=Pa&&(X+=Rt),E.startsWith("_"))m+=s[0];else{let Bt=B.classNameAliases[E]||E;R(s[0],Bt)}}else m+=s[0];o=i.keywordPatternRe.lastIndex,s=i.keywordPatternRe.exec(C)}m+=C.substring(o),A.addText(m)}function V(){if(C==="")return;let o=null;if(typeof i.subLanguage=="string"){if(!t[i.subLanguage]){A.addText(C);return}o=_(i.subLanguage,C,!0,Ie[i.subLanguage]),Ie[i.subLanguage]=o._top}else o=Y(C,i.subLanguage.length?i.subLanguage:null);i.relevance>0&&(X+=o.relevance),A.__addSublanguage(o._emitter,o.language)}function P(){i.subLanguage!=null?V():O(),C=""}function R(o,s){o!==""&&(A.startScope(s),A.addText(o),A.endScope())}function Ue(o,s){let m=1,h=s.length-1;for(;m<=h;){if(!o._emit[m]){m++;continue}let k=B.classNameAliases[o[m]]||o[m],E=s[m];k?R(E,k):(C=E,O(),C=""),m++}}function He(o,s){return o.scope&&typeof o.scope=="string"&&A.openNode(B.classNameAliases[o.scope]||o.scope),o.beginScope&&(o.beginScope._wrap?(R(C,B.classNameAliases[o.beginScope._wrap]||o.beginScope._wrap),C=""):o.beginScope._multi&&(Ue(o.beginScope,s),C="")),i=Object.create(o,{parent:{value:i}}),i}function _e(o,s,m){let h=Gt(o.endRe,m);if(h){if(o["on:end"]){let k=new K(o);o["on:end"](s,k),k.isMatchIgnored&&(h=!1)}if(h){for(;o.endsParent&&o.parent;)o=o.parent;return o}}if(o.endsWithParent)return _e(o.parent,s,m)}function bt(o){return i.matcher.regexIndex===0?(C+=o[0],1):(re=!0,0)}function At(o){let s=o[0],m=o.rule,h=new K(m),k=[m.__beforeBegin,m["on:begin"]];for(let E of k)if(E&&(E(o,h),h.isMatchIgnored))return bt(s);return m.skip?C+=s:(m.excludeBegin&&(C+=s),P(),!m.returnBegin&&!m.excludeBegin&&(C=s)),He(m,o),m.returnBegin?0:s.length}function kt(o){let s=o[0],m=u.substring(o.index),h=_e(i,o,m);if(!h)return je;let k=i;i.endScope&&i.endScope._wrap?(P(),R(s,i.endScope._wrap)):i.endScope&&i.endScope._multi?(P(),Ue(i.endScope,o)):k.skip?C+=s:(k.returnEnd||k.excludeEnd||(C+=s),P(),k.excludeEnd&&(C=s));do i.scope&&A.closeNode(),!i.skip&&!i.subLanguage&&(X+=i.relevance),i=i.parent;while(i!==h.parent);return h.starts&&He(h.starts,o),k.returnEnd?0:s.length}function Pt(){let o=[];for(let s=i;s!==B;s=s.parent)s.scope&&o.unshift(s.scope);o.forEach(s=>A.openNode(s))}let z={};function qe(o,s){let m=s&&s[0];if(C+=o,m==null)return P(),0;if(z.type==="begin"&&s.type==="end"&&z.index===s.index&&m===""){if(C+=u.slice(s.index,s.index+1),!c){let h=new Error(`0 width match regex (${r})`);throw h.languageName=r,h.badRule=z.rule,h}return 1}if(z=s,s.type==="begin")return At(s);if(s.type==="illegal"&&!x){let h=new Error('Illegal lexeme "'+m+'" for mode "'+(i.scope||"<unnamed>")+'"');throw h.mode=i,h}else if(s.type==="end"){let h=kt(s);if(h!==je)return h}if(s.type==="illegal"&&m==="")return C+=`
`,1;if(ae>1e5&&ae>s.index*3)throw new Error("potential infinite loop, way more iterations than matches");return C+=m,m.length}let B=D(r);if(!B)throw F(S.replace("{}",r)),new Error('Unknown language: "'+r+'"');let yt=ba(B),te="",i=g||yt,Ie={},A=new f.__emitter(f);Pt();let C="",X=0,T=0,ae=0,re=!1;try{if(B.__emitTokens)B.__emitTokens(u,A);else{for(i.matcher.considerAll();;){ae++,re?re=!1:i.matcher.considerAll(),i.matcher.lastIndex=T;let o=i.matcher.exec(u);if(!o)break;let s=u.substring(T,o.index),m=qe(s,o);T=o.index+m}qe(u.substring(T))}return A.finalize(),te=A.toHTML(),{language:r,value:te,relevance:X,illegal:!1,_emitter:A,_top:i}}catch(o){if(o.message&&o.message.includes("Illegal"))return{language:r,value:oe(u),illegal:!0,relevance:0,_illegalBy:{message:o.message,index:T,context:u.slice(T-100,T+100),mode:o.mode,resultSoFar:te},_emitter:A};if(c)return{language:r,value:oe(u),illegal:!1,relevance:0,errorRaised:o,_emitter:A,_top:i};throw o}}function Q(r){let u={value:oe(r),illegal:!1,relevance:0,_top:l,_emitter:new f.__emitter(f)};return u._emitter.addText(r),u}function Y(r,u){u=u||f.languages||Object.keys(t);let x=Q(r),g=u.filter(D).filter(ve).map(P=>_(P,r,!1));g.unshift(x);let M=g.sort((P,R)=>{if(P.relevance!==R.relevance)return R.relevance-P.relevance;if(P.language&&R.language){if(D(P.language).supersetOf===R.language)return 1;if(D(R.language).supersetOf===P.language)return-1}return 0}),[y,O]=M,V=y;return V.secondBest=O,V}function nt(r,u,x){let g=u&&a[u]||x;r.classList.add("hljs"),r.classList.add(`language-${g}`)}function ee(r){let u=null,x=b(r);if(p(x))return;if(W("before:highlightElement",{el:r,language:x}),r.dataset.highlighted){console.log("Element previously highlighted. To highlight again, first unset `dataset.highlighted`.",r);return}if(r.children.length>0&&(f.ignoreUnescapedHTML||(console.warn("One of your code blocks includes unescaped HTML. This is a potentially serious security risk."),console.warn("https://github.com/highlightjs/highlight.js/wiki/security"),console.warn("The element with unescaped HTML:"),console.warn(r)),f.throwUnescapedHTML))throw new se("One of your code blocks includes unescaped HTML.",r.innerHTML);u=r;let g=u.textContent,M=x?w(g,{language:x,ignoreIllegals:!0}):Y(g);r.innerHTML=M.value,r.dataset.highlighted="yes",nt(r,x,M.language),r.result={language:M.language,re:M.relevance,relevance:M.relevance},M.secondBest&&(r.secondBest={language:M.secondBest.language,relevance:M.secondBest.relevance}),W("after:highlightElement",{el:r,result:M,text:g})}function it(r){f=Xe(f,r)}let mt=()=>{G(),U("10.6.0","initHighlighting() deprecated.  Use highlightAll() now.")};function xt(){G(),U("10.6.0","initHighlightingOnLoad() deprecated.  Use highlightAll() now.")}let Te=!1;function G(){function r(){G()}if(document.readyState==="loading"){Te||window.addEventListener("DOMContentLoaded",r,!1),Te=!0;return}document.querySelectorAll(f.cssSelector).forEach(ee)}function ct(r,u){let x=null;try{x=u(e)}catch(g){if(F("Language definition for '{}' could not be registered.".replace("{}",r)),c)F(g);else throw g;x=l}x.name||(x.name=r),t[r]=x,x.rawDefinition=u.bind(null,e),x.aliases&&Fe(x.aliases,{languageName:r})}function ht(r){delete t[r];for(let u of Object.keys(a))a[u]===r&&delete a[u]}function gt(){return Object.keys(t)}function D(r){return r=(r||"").toLowerCase(),t[r]||t[a[r]]}function Fe(r,{languageName:u}){typeof r=="string"&&(r=[r]),r.forEach(x=>{a[x.toLowerCase()]=u})}function ve(r){let u=D(r);return u&&!u.disableAutodetect}function Ct(r){r["before:highlightBlock"]&&!r["before:highlightElement"]&&(r["before:highlightElement"]=u=>{r["before:highlightBlock"](Object.assign({block:u.el},u))}),r["after:highlightBlock"]&&!r["after:highlightElement"]&&(r["after:highlightElement"]=u=>{r["after:highlightBlock"](Object.assign({block:u.el},u))})}function St(r){Ct(r),d.push(r)}function wt(r){let u=d.indexOf(r);u!==-1&&d.splice(u,1)}function W(r,u){let x=r;d.forEach(function(g){g[x]&&g[x](u)})}function Mt(r){return U("10.7.0","highlightBlock will be removed entirely in v12.0"),U("10.7.0","Please use highlightElement now."),ee(r)}Object.assign(e,{highlight:w,highlightAuto:Y,highlightAll:G,highlightElement:ee,highlightBlock:Mt,configure:it,initHighlighting:mt,initHighlightingOnLoad:xt,registerLanguage:ct,unregisterLanguage:ht,listLanguages:gt,getLanguage:D,registerAliases:Fe,autoDetection:ve,inherit:Xe,addPlugin:St,removePlugin:wt}),e.debugMode=function(){c=!1},e.safeMode=function(){c=!0},e.versionString=ka,e.regex={concat:v,lookahead:Ze,either:ue,optional:It,anyNumberOfTimes:qt};for(let r in j)typeof j[r]=="object"&&Ke(j[r]);return Object.assign(e,j),e},H=ft({});H.newInstance=()=>ft({});lt.exports=H;H.HighlightJS=H;H.default=H});var ut=Ut(st(),1);var dt=ut.default;var pt=(e,t,a=[])=>{let d=document.createElementNS("http://www.w3.org/2000/svg",e);return Object.keys(t).forEach(c=>{d.setAttribute(c,String(t[c]))}),a.length&&a.forEach(c=>{let S=pt(...c);d.appendChild(S)}),d},ne=([e,t,a])=>pt(e,t,a);var n={xmlns:"http://www.w3.org/2000/svg",width:24,height:24,viewBox:"0 0 24 24",fill:"none",stroke:"currentColor","stroke-width":2,"stroke-linecap":"round","stroke-linejoin":"round"};var ie=["svg",n,[["path",{d:"m12 19-7-7 7-7"}],["path",{d:"M19 12H5"}]]];var me=["svg",n,[["path",{d:"M5 12h14"}],["path",{d:"m12 5 7 7-7 7"}]]];var xe=["svg",n,[["path",{d:"M20 6 9 17l-5-5"}]]];var ce=["svg",n,[["path",{d:"M15 2H9a1 1 0 0 0-1 1v2c0 .6.4 1 1 1h6c.6 0 1-.4 1-1V3c0-.6-.4-1-1-1Z"}],["path",{d:"M8 4H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2M16 4h2a2 2 0 0 1 2 2v2M11 14h10"}],["path",{d:"m17 10 4 4-4 4"}]]];var he=["svg",n,[["rect",{width:"14",height:"14",x:"8",y:"8",rx:"2",ry:"2"}],["path",{d:"M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"}]]];var ge=["svg",n,[["path",{d:"M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"}],["polyline",{points:"7 10 12 15 17 10"}],["line",{x1:"12",x2:"12",y1:"15",y2:"3"}]]];var Ce=["svg",n,[["path",{d:"M10.733 5.076a10.744 10.744 0 0 1 11.205 6.575 1 1 0 0 1 0 .696 10.747 10.747 0 0 1-1.444 2.49"}],["path",{d:"M14.084 14.158a3 3 0 0 1-4.242-4.242"}],["path",{d:"M17.479 17.499a10.75 10.75 0 0 1-15.417-5.151 1 1 0 0 1 0-.696 10.75 10.75 0 0 1 4.446-5.143"}],["path",{d:"m2 2 20 20"}]]];var Se=["svg",n,[["path",{d:"M2.062 12.348a1 1 0 0 1 0-.696 10.75 10.75 0 0 1 19.876 0 1 1 0 0 1 0 .696 10.75 10.75 0 0 1-19.876 0"}],["circle",{cx:"12",cy:"12",r:"3"}]]];var we=["svg",n,[["path",{d:"M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"}],["path",{d:"M14 2v4a2 2 0 0 0 2 2h4"}],["path",{d:"M10 9H8"}],["path",{d:"M16 13H8"}],["path",{d:"M16 17H8"}]]];var Me=["svg",n,[["path",{d:"m6 14 1.5-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.54 6a2 2 0 0 1-1.95 1.5H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H18a2 2 0 0 1 2 2v2"}]]];var be=["svg",n,[["path",{d:"M8 3H5a2 2 0 0 0-2 2v3"}],["path",{d:"M21 8V5a2 2 0 0 0-2-2h-3"}],["path",{d:"M3 16v3a2 2 0 0 0 2 2h3"}],["path",{d:"M16 21h3a2 2 0 0 0 2-2v-3"}]]];var Ae=["svg",n,[["path",{d:"M8 3v3a2 2 0 0 1-2 2H3"}],["path",{d:"M21 8h-3a2 2 0 0 1-2-2V3"}],["path",{d:"M3 16h3a2 2 0 0 1 2 2v3"}],["path",{d:"M16 21v-3a2 2 0 0 1 2-2h3"}]]];var J=["svg",n,[["rect",{width:"18",height:"18",x:"3",y:"3",rx:"2"}],["path",{d:"M3 9h18"}],["path",{d:"M9 21V9"}]]];var ke=["svg",n,[["polygon",{points:"6 3 20 12 6 21 6 3"}]]];var Pe=["svg",n,[["path",{d:"M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"}],["path",{d:"M3 3v5h5"}]]];var ye=["svg",n,[["circle",{cx:"11",cy:"11",r:"8"}],["path",{d:"m21 21-4.3-4.3"}]]];var Re=["svg",n,[["rect",{width:"20",height:"8",x:"2",y:"2",rx:"2",ry:"2"}],["rect",{width:"20",height:"8",x:"2",y:"14",rx:"2",ry:"2"}],["line",{x1:"6",x2:"6.01",y1:"6",y2:"6"}],["line",{x1:"6",x2:"6.01",y1:"18",y2:"18"}]]];var Be=["svg",n,[["path",{d:"M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"}],["circle",{cx:"12",cy:"12",r:"3"}]]];var Ee=["svg",n,[["polyline",{points:"4 17 10 11 4 5"}],["line",{x1:"12",x2:"20",y1:"19",y2:"19"}]]];var De=["svg",n,[["path",{d:"M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"}],["polyline",{points:"17 8 12 3 7 8"}],["line",{x1:"12",x2:"12",y1:"3",y2:"15"}]]];var Oe=["svg",n,[["line",{x1:"3",x2:"21",y1:"6",y2:"6"}],["path",{d:"M3 12h15a3 3 0 1 1 0 6h-4"}],["polyline",{points:"16 16 14 18 16 20"}],["line",{x1:"3",x2:"10",y1:"18",y2:"18"}]]];var Le=["svg",n,[["path",{d:"M18 6 6 18"}],["path",{d:"m6 6 12 12"}]]];window.LogHighlight=dt;window.LogIcons={createElement:ne,FileText:we,FolderOpen:Me,Upload:De,Server:Re,ClipboardPaste:ce,Settings:Be,Download:ge,Search:ye,RotateCcw:Pe,Copy:he,Maximize:be,Minimize:Ae,ArrowLeft:ie,ArrowRight:me,Play:ke,X:Le,PanelsTopLeft:J,Check:xe,Terminal:Ee,WrapText:Oe,Eye:Se,EyeOff:Ce};})();
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

lucide/dist/esm/icons/eye-off.js:
  (**
   * @license lucide v0.468.0 - ISC
   *
   * This source code is licensed under the ISC license.
   * See the LICENSE file in the root directory of this source tree.
   *)

lucide/dist/esm/icons/eye.js:
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
