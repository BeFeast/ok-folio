import Foundation
#if canImport(FoundationNetworking)
import FoundationNetworking
#endif

/// URLProtocol stub: every request routed through a session whose
/// configuration lists this class is answered by `handler` — no live network.
final class StubURLProtocol: URLProtocol {
    struct StubResponse {
        var status: Int
        var data: Data
        var headers: [String: String]

        init(status: Int = 200, data: Data = Data(), headers: [String: String] = ["Content-Type": "application/json"]) {
            self.status = status
            self.data = data
            self.headers = headers
        }
    }

    // Plain static storage: tests in this suite run their requests serially,
    // and Swift 5.9 has no `nonisolated(unsafe)` yet.

    /// Set per test. Receives the outgoing request, returns the canned reply.
    static var handler: ((URLRequest) throws -> StubResponse)?
    /// Every request seen by the stub, in order.
    static var recorded: [URLRequest] = []
    /// Captured request bodies (URLSession may expose the body only as a
    /// stream inside URLProtocol, so it is drained eagerly here).
    static var recordedBodies: [Data] = []
    /// When true, requests are recorded but never answered — for exercising
    /// cancellation of in-flight tasks.
    static var hang = false

    static func reset() {
        handler = nil
        recorded = []
        recordedBodies = []
        hang = false
    }

    override class func canInit(with request: URLRequest) -> Bool { true }

    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        Self.recorded.append(request)
        Self.recordedBodies.append(Self.drainBody(of: request))

        if Self.hang {
            return
        }

        guard let handler = Self.handler else {
            client?.urlProtocol(self, didFailWithError: URLError(.unsupportedURL))
            return
        }
        do {
            let stub = try handler(request)
            guard let url = request.url,
                  let response = HTTPURLResponse(
                      url: url,
                      statusCode: stub.status,
                      httpVersion: "HTTP/1.1",
                      headerFields: stub.headers
                  )
            else {
                client?.urlProtocol(self, didFailWithError: URLError(.badServerResponse))
                return
            }
            client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
            client?.urlProtocol(self, didLoad: stub.data)
            client?.urlProtocolDidFinishLoading(self)
        } catch {
            client?.urlProtocol(self, didFailWithError: error)
        }
    }

    override func stopLoading() {}

    private static func drainBody(of request: URLRequest) -> Data {
        if let body = request.httpBody {
            return body
        }
        guard let stream = request.httpBodyStream else {
            return Data()
        }
        stream.open()
        defer { stream.close() }
        var data = Data()
        let bufferSize = 4096
        var buffer = [UInt8](repeating: 0, count: bufferSize)
        while stream.hasBytesAvailable {
            let read = stream.read(&buffer, maxLength: bufferSize)
            if read <= 0 { break }
            data.append(buffer, count: read)
        }
        return data
    }
}
