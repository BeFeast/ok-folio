import Foundation

/// Errors surfaced by `FolioClient`.
///
/// - `httpStatus`: the server answered with a non-2xx status. `body` carries the
///   raw response body (the Go API sends `{"error": "message"}` envelopes).
/// - `decoding`: the response body could not be decoded into the expected model.
///   `path` names the endpoint path that produced the payload.
/// - `transport`: the request never produced an HTTP response (connection
///   refused, DNS failure, timeout, ...).
public enum FolioError: Error {
    case httpStatus(Int, body: String)
    case decoding(Error, path: String)
    case transport(Error)
}

extension FolioError: CustomStringConvertible {
    public var description: String {
        switch self {
        case .httpStatus(let status, let body):
            return "HTTP \(status): \(body)"
        case .decoding(let error, let path):
            return "Decoding failed for \(path): \(error)"
        case .transport(let error):
            return "Transport error: \(error)"
        }
    }
}

/// `error.localizedDescription` is what UIs show; without this conformance it
/// degrades to a generic NSError string that hides the server's message.
extension FolioError: LocalizedError {
    public var errorDescription: String? {
        switch self {
        case .httpStatus(let status, let body):
            // The Go API wraps errors as {"error": "message"} — surface the
            // message itself when present.
            if let data = body.data(using: .utf8),
               let envelope = try? JSONDecoder().decode([String: String].self, from: data),
               let message = envelope["error"], !message.isEmpty {
                return message
            }
            let trimmed = body.trimmingCharacters(in: .whitespacesAndNewlines)
            return trimmed.isEmpty ? "HTTP \(status)" : "HTTP \(status): \(trimmed.prefix(200))"
        case .decoding(_, let path):
            return "Unexpected response from \(path)"
        case .transport(let error):
            return error.localizedDescription
        }
    }
}
