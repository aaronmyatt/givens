import Foundation
import Testing

/// Every capability in capabilities/ either has a Swift suite here or a reason it has none,
/// so a new capability can't slip past the app's suite unnoticed.
@Suite("Coverage") @MainActor struct CoverageTests {
    /// Capabilities with a suite in this target: each suite lives in a folder named after its
    /// capability, next to this file.
    static var covered: Set<String> {
        let here = URL(fileURLWithPath: #filePath).deletingLastPathComponent()
        let folders = (try? FileManager.default.contentsOfDirectory(atPath: here.path)) ?? []
        // Folders here are suites, except World/, which they share.
        return Set(folders.filter { $0 != "World" && !$0.hasSuffix(".swift") })
    }

    /// Capabilities no window is part of. The Go suite in acceptance/ covers them.
    static let windowless: [String: String] = [
        "the-kit": "the kit is bash run in a terminal; its feature says none need the app",
        "use-givens-from-the-terminal": "the givens command, run without opening the app",
    ]

    @Test func everyCapabilityIsCoveredOrExcused() throws {
        let capabilities = Toolchain.root.appendingPathComponent("capabilities")
        let folders = try FileManager.default.contentsOfDirectory(atPath: capabilities.path)
        let features = folders.filter { folder in
            FileManager.default.fileExists(
                atPath: capabilities.appendingPathComponent("\(folder)/\(folder).feature").path)
        }
        for folder in features where !Self.covered.contains(folder) {
            #expect(Self.windowless[folder] != nil, "\(folder) has no Swift suite and no reason")
        }
        for folder in Self.covered.union(Self.windowless.keys) {
            #expect(features.contains(folder), "\(folder) is listed but isn't a capability")
        }
    }
}
