const AUTH_COUNTRY_SELECTION_KEY = "xingyu_auth_country_selection";

export interface SelectedCountry {
  from?: string;
  name?: string;
  tel: string;
}

export function saveSelectedCountry(country: SelectedCountry) {
  uni.setStorageSync(AUTH_COUNTRY_SELECTION_KEY, country);
}

export function takeSelectedCountry(from: string) {
  const country = uni.getStorageSync(AUTH_COUNTRY_SELECTION_KEY) as SelectedCountry | "";
  if (!country || country.from !== from || !country.tel) {
    return undefined;
  }
  uni.removeStorageSync(AUTH_COUNTRY_SELECTION_KEY);
  return country;
}
