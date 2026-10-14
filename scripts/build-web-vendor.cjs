// npm install --no-save highlight.js@11.11.1 lucide@0.468.0 esbuild
const path=require('path');
const esbuild=require('esbuild');
esbuild.build({
 stdin:{contents:`import hljs from 'highlight.js/lib/core';
 import {createElement,FileText,FolderOpen,Upload,Server,ClipboardPaste,Settings,Download,Search,RotateCcw,Copy,Maximize,Minimize,ArrowLeft,ArrowRight,Play,X,PanelsTopLeft,Check,Terminal,WrapText} from 'lucide';
 window.LogHighlight=hljs;
 window.LogIcons={createElement,FileText,FolderOpen,Upload,Server,ClipboardPaste,Settings,Download,Search,RotateCcw,Copy,Maximize,Minimize,ArrowLeft,ArrowRight,Play,X,PanelsTopLeft,Check,Terminal,WrapText};`,resolveDir:path.resolve(__dirname,'..'),loader:'js'},
 nodePaths:process.env.LR_VENDOR_MODULES?[process.env.LR_VENDOR_MODULES]:[],
 outfile:path.resolve(__dirname,'../internal/server/web/vendor/log-components.js'),
 bundle:true,minify:true,format:'iife',target:['es2020'],legalComments:'eof'
}).catch(e=>{console.error(e);process.exitCode=1});
