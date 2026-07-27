import MD5 from "crypto-js/md5";

export function md5(value: string) {
  return MD5(value).toString();
}
