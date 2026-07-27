function e(e){const t=Number(e||0);return Number.isFinite(t)?t>=1e4?`${(t/1e4).toFixed(t>=1e5?0:1)}万`:String(t):"0"}function t(e){return new Promise(t=>setTimeout(t,e))}export{e as d,t as s};
