import DCloudUTSFoundation
@objc(UTSSDKModulesClawRustdeskHostRemoteHostStatus)
@objcMembers
public class RemoteHostStatus : NSObject, UTSObject {
    public var available: Bool = false
    public var configured: Bool = false
    public var running: Bool = false
    public var device_code: String?
    public var service_status: String!
    public var message: String?
    public var permissions: UTSJSONObject!
    public subscript(_ key: String) -> Any? {
        get {
            return utsSubscriptGetValue(key)
        }
        set {
            switch(key){
                case "available":
                    self.available = try! utsSubscriptCheckValue(newValue)
                case "configured":
                    self.configured = try! utsSubscriptCheckValue(newValue)
                case "running":
                    self.running = try! utsSubscriptCheckValue(newValue)
                case "device_code":
                    self.device_code = try! utsSubscriptCheckValueIfPresent(newValue)
                case "service_status":
                    self.service_status = try! utsSubscriptCheckValue(newValue)
                case "message":
                    self.message = try! utsSubscriptCheckValueIfPresent(newValue)
                case "permissions":
                    self.permissions = try! utsSubscriptCheckValue(newValue)
                default:
                    break
            }
        }
    }
    public override init() {
        super.init()
    }
    public init(_ obj: UTSJSONObject) {
        self.available = obj["available"] as! Bool
        self.configured = obj["configured"] as! Bool
        self.running = obj["running"] as! Bool
        self.device_code = obj["device_code"] as! String?
        self.service_status = obj["service_status"] as! String
        self.message = obj["message"] as! String?
        self.permissions = obj["permissions"] as! UTSJSONObject
    }
}
public typealias RemoteHostCallback = (_ status: RemoteHostStatus) -> Void
public var unsupported = {
() -> UTSJSONObject in
return (UTSJSONObject([
    "available": false,
    "configured": false,
    "running": false,
    "service_status": "unsupported",
    "message": "远程协助仅支持 Android 原生安装包",
    "permissions": UTSJSONObject([:])
]))
}
public func initialize(_ _options: UTSJSONObject, _ callback: @escaping RemoteHostCallback) -> Void {
    callback(unsupported())
}
public func start(_ callback: @escaping RemoteHostCallback) -> Void {
    callback(unsupported())
}
public func stop(_ _options: UTSJSONObject, _ callback: @escaping RemoteHostCallback) -> Void {
    callback(unsupported())
}
public func getStatus(_ callback: @escaping RemoteHostCallback) -> Void {
    callback(unsupported())
}
public func openPermissionSettings(_ _permission: String, _ callback: @escaping RemoteHostCallback) -> Void {
    callback(unsupported())
}
public func initializeByJs(_ _options: UTSJSONObject, _ callback: UTSCallback) -> Void {
    return initialize(_options, {
    (status: RemoteHostStatus) -> Void in
    callback(status)
    })
}
public func startByJs(_ callback: UTSCallback) -> Void {
    return start({
    (status: RemoteHostStatus) -> Void in
    callback(status)
    })
}
public func stopByJs(_ _options: UTSJSONObject, _ callback: UTSCallback) -> Void {
    return stop(_options, {
    (status: RemoteHostStatus) -> Void in
    callback(status)
    })
}
public func getStatusByJs(_ callback: UTSCallback) -> Void {
    return getStatus({
    (status: RemoteHostStatus) -> Void in
    callback(status)
    })
}
public func openPermissionSettingsByJs(_ _permission: String, _ callback: UTSCallback) -> Void {
    return openPermissionSettings(_permission, {
    (status: RemoteHostStatus) -> Void in
    callback(status)
    })
}
@objc(UTSSDKModulesClawRustdeskHostIndexSwift)
@objcMembers
public class UTSSDKModulesClawRustdeskHostIndexSwift : NSObject {
    public static func s_initializeByJs(_ _options: UTSJSONObject, _ callback: UTSCallback) -> Void {
        return initializeByJs(_options, callback)
    }
    public static func s_startByJs(_ callback: UTSCallback) -> Void {
        return startByJs(callback)
    }
    public static func s_stopByJs(_ _options: UTSJSONObject, _ callback: UTSCallback) -> Void {
        return stopByJs(_options, callback)
    }
    public static func s_getStatusByJs(_ callback: UTSCallback) -> Void {
        return getStatusByJs(callback)
    }
    public static func s_openPermissionSettingsByJs(_ _permission: String, _ callback: UTSCallback) -> Void {
        return openPermissionSettingsByJs(_permission, callback)
    }
}
