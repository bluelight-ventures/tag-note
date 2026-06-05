import Foundation
import SwiftUI

struct User: Codable, Equatable {
    let id: String
    let email: String
    let displayName: String
    let createdAt: Date
    let emailVerified: Bool
    let hasPassword: Bool
    let hasGoogle: Bool
    let hasApple: Bool

    init(id: String, email: String, displayName: String, createdAt: Date, emailVerified: Bool, hasPassword: Bool, hasGoogle: Bool, hasApple: Bool = false) {
        self.id = id
        self.email = email
        self.displayName = displayName
        self.createdAt = createdAt
        self.emailVerified = emailVerified
        self.hasPassword = hasPassword
        self.hasGoogle = hasGoogle
        self.hasApple = hasApple
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        id = try container.decodeIfPresent(String.self, forKey: .id) ?? ""
        email = try container.decodeIfPresent(String.self, forKey: .email) ?? ""
        displayName = try container.decodeIfPresent(String.self, forKey: .displayName) ?? email
        createdAt = try container.decodeIfPresent(Date.self, forKey: .createdAt) ?? Date(timeIntervalSince1970: 0)
        emailVerified = try container.decodeIfPresent(Bool.self, forKey: .emailVerified) ?? false
        hasPassword = try container.decodeIfPresent(Bool.self, forKey: .hasPassword) ?? true
        hasGoogle = try container.decodeIfPresent(Bool.self, forKey: .hasGoogle) ?? false
        hasApple = try container.decodeIfPresent(Bool.self, forKey: .hasApple) ?? false
    }
}

struct AuthResponse: Codable {
    let token: String?
    let user: User?
    let pendingVerify: Bool?
    let pendingVerifyEmail: String?
}

struct SubNote: Codable, Identifiable, Equatable {
    let id: String
    let shortID: String
    var content: String
    var snippet: String?
    let createdAt: Date
    var updatedAt: Date?
    var tags: [String]
    var pinned: Bool

    var routeID: String { shortID.isEmpty ? id : shortID }
    var displayDate: Date { updatedAt ?? createdAt }

    enum CodingKeys: String, CodingKey {
        case id
        case shortID = "shortId"
        case content
        case snippet
        case createdAt
        case updatedAt
        case tags
        case pinned
    }
}

struct CreateNoteResponse: Codable {
    let id: String
    let shortID: String
    let createdAt: Date

    enum CodingKeys: String, CodingKey {
        case id
        case shortID = "shortId"
        case createdAt
    }
}

struct CreateNoteRequest: Encodable {
    let content: String
    let tags: [String]
}

struct UpdateNoteRequest: Encodable {
    let content: String?
    let tags: [String]?
}

struct TagInfo: Codable, Identifiable, Equatable {
    let name: String
    var status: String
    var noteCount: Int
    var importance: Int
    var urgency: Int

    var id: String { name }
    var isUnreviewed: Bool { status == "unreviewed" || status == "pending" }
}

struct TagRenameRequest: Encodable {
    let newName: String
}

struct TagPriorityRequest: Encodable {
    let importance: Int?
    let urgency: Int?
}

struct TagNoteSettings: Codable, Equatable {
    var theme: String
    var previewMode: String
    var noteWidth: String

    static let fallback = TagNoteSettings(theme: "everforest-light", previewMode: "plain", noteWidth: "")
}

struct APIErrorResponse: Decodable {
    let error: String?
    let message: String?
}

enum PreviewMode: String, CaseIterable, Identifiable {
    case write
    case preview

    var id: String { rawValue }
    var label: String { rawValue.capitalized }
}

enum SaveStatus: Equatable {
    case idle
    case unsaved
    case saving
    case saved
    case failed(String)

    var label: String {
        switch self {
        case .idle:
            return ""
        case .unsaved:
            return "Unsaved"
        case .saving:
            return "Saving..."
        case .saved:
            return "Saved"
        case .failed(let message):
            return message
        }
    }
}

enum Theme: String, CaseIterable, Identifiable {
    case everforestLight = "everforest-light"
    case everforestDark = "everforest-dark"
    case solarizedLight = "solarized-light"
    case solarizedDark = "solarized-dark"
    case gruvboxLight = "gruvbox-light"
    case gruvboxDark = "gruvbox-dark"
    case nordLight = "nord-light"
    case nordDark = "nord-dark"

    var id: String { rawValue }

    var label: String {
        switch self {
        case .everforestLight: return "Everforest Light"
        case .everforestDark: return "Everforest Dark"
        case .solarizedLight: return "Solarized Light"
        case .solarizedDark: return "Solarized Dark"
        case .gruvboxLight: return "Gruvbox Light"
        case .gruvboxDark: return "Gruvbox Dark"
        case .nordLight: return "Nord Light"
        case .nordDark: return "Nord Dark"
        }
    }
}

struct TagNotePalette {
    let background: Color
    let card: Color
    let border: Color
    let text: Color
    let secondaryText: Color
    let accent: Color
    let tagBackground: Color
    let destructive: Color
    /// Semantic success (approved / saved). Maps to the web `--green` token.
    let success: Color
    /// Semantic warning (unsaved / saving / important). Maps to the web `--mark-text` amber token.
    let warning: Color
    /// Semantic info (links / read-more / rename). Maps to the web `--blue` token.
    let info: Color
    /// Whether this theme is a dark variant. Drives priority color math, like the web client.
    let isDark: Bool

    static func palette(for theme: String) -> TagNotePalette {
        switch Theme(rawValue: theme) {
        case .everforestDark:
            return TagNotePalette(background: Color(hex: 0x272E33), card: Color(hex: 0x2D353B), border: Color(hex: 0x374145), text: Color(hex: 0xD3C6AA), secondaryText: Color(hex: 0x9DA9A0), accent: Color(hex: 0xA7C080), tagBackground: Color(hex: 0x374145), destructive: Color(hex: 0xE67E80), success: Color(hex: 0xA7C080), warning: Color(hex: 0xDBBC7F), info: Color(hex: 0x7FBBB3), isDark: true)
        case .solarizedLight:
            return TagNotePalette(background: Color(hex: 0xEEE8D5), card: Color(hex: 0xFDF6E3), border: Color(hex: 0xD3CBB7), text: Color(hex: 0x657B83), secondaryText: Color(hex: 0x839496), accent: Color(hex: 0x268BD2), tagBackground: Color(hex: 0xE6DFCC), destructive: Color(hex: 0xDC322F), success: Color(hex: 0x859900), warning: Color(hex: 0xB58900), info: Color(hex: 0x268BD2), isDark: false)
        case .solarizedDark:
            return TagNotePalette(background: Color(hex: 0x002B36), card: Color(hex: 0x073642), border: Color(hex: 0x0A404D), text: Color(hex: 0x839496), secondaryText: Color(hex: 0x586E75), accent: Color(hex: 0x268BD2), tagBackground: Color(hex: 0x0A404D), destructive: Color(hex: 0xDC322F), success: Color(hex: 0x859900), warning: Color(hex: 0xB58900), info: Color(hex: 0x268BD2), isDark: true)
        case .gruvboxLight:
            return TagNotePalette(background: Color(hex: 0xF2E5BC), card: Color(hex: 0xFBF1C7), border: Color(hex: 0xD5C4A1), text: Color(hex: 0x3C3836), secondaryText: Color(hex: 0x7C6F64), accent: Color(hex: 0x79740E), tagBackground: Color(hex: 0xEBDBB2), destructive: Color(hex: 0xCC241D), success: Color(hex: 0x98971A), warning: Color(hex: 0xD79921), info: Color(hex: 0x458588), isDark: false)
        case .gruvboxDark:
            return TagNotePalette(background: Color(hex: 0x282828), card: Color(hex: 0x3C3836), border: Color(hex: 0x504945), text: Color(hex: 0xEBDBB2), secondaryText: Color(hex: 0xA89984), accent: Color(hex: 0xB8BB26), tagBackground: Color(hex: 0x504945), destructive: Color(hex: 0xFB4934), success: Color(hex: 0xB8BB26), warning: Color(hex: 0xFABD2F), info: Color(hex: 0x83A598), isDark: true)
        case .nordLight:
            return TagNotePalette(background: Color(hex: 0xE5E9F0), card: Color(hex: 0xECEFF4), border: Color(hex: 0xD8DEE9), text: Color(hex: 0x2E3440), secondaryText: Color(hex: 0x4C566A), accent: Color(hex: 0x5E81AC), tagBackground: Color(hex: 0xD8DEE9), destructive: Color(hex: 0xBF616A), success: Color(hex: 0xA3BE8C), warning: Color(hex: 0xD08770), info: Color(hex: 0x5E81AC), isDark: false)
        case .nordDark:
            return TagNotePalette(background: Color(hex: 0x2E3440), card: Color(hex: 0x3B4252), border: Color(hex: 0x434C5E), text: Color(hex: 0xECEFF4), secondaryText: Color(hex: 0xD8DEE9), accent: Color(hex: 0x88C0D0), tagBackground: Color(hex: 0x434C5E), destructive: Color(hex: 0xBF616A), success: Color(hex: 0xA3BE8C), warning: Color(hex: 0xEBCB8B), info: Color(hex: 0x81A1C1), isDark: true)
        default:
            return TagNotePalette(background: Color(hex: 0xF3EAD3), card: Color(hex: 0xFDF6E3), border: Color(hex: 0xD5C4A1), text: Color(hex: 0x5C6A72), secondaryText: Color(hex: 0x829181), accent: Color(hex: 0x8DA101), tagBackground: Color(hex: 0xEAE2CC), destructive: Color(hex: 0xF85552), success: Color(hex: 0x8DA101), warning: Color(hex: 0xDFA44A), info: Color(hex: 0x3A94C5), isDark: false)
        }
    }
}

extension Color {
    init(hex: UInt, alpha: Double = 1) {
        self.init(
            .sRGB,
            red: Double((hex >> 16) & 0xFF) / 255,
            green: Double((hex >> 8) & 0xFF) / 255,
            blue: Double(hex & 0xFF) / 255,
            opacity: alpha
        )
    }

    /// CSS-style HSL constructor. `hue` in degrees (0–360); `saturation`/`lightness` in 0–1.
    /// Used to mirror the web client's priority color math exactly.
    init(hslHue hue: Double, saturation: Double, lightness: Double) {
        let c = (1 - abs(2 * lightness - 1)) * saturation
        let hp = (hue.truncatingRemainder(dividingBy: 360) + 360).truncatingRemainder(dividingBy: 360) / 60
        let x = c * (1 - abs(hp.truncatingRemainder(dividingBy: 2) - 1))
        let (r1, g1, b1): (Double, Double, Double)
        switch hp {
        case 0..<1: (r1, g1, b1) = (c, x, 0)
        case 1..<2: (r1, g1, b1) = (x, c, 0)
        case 2..<3: (r1, g1, b1) = (0, c, x)
        case 3..<4: (r1, g1, b1) = (0, x, c)
        case 4..<5: (r1, g1, b1) = (x, 0, c)
        default: (r1, g1, b1) = (c, 0, x)
        }
        let m = lightness - c / 2
        self.init(.sRGB, red: r1 + m, green: g1 + m, blue: b1 + m, opacity: 1)
    }
}

/// Themed Importance × Urgency color mapping, ported from the web client's
/// `getPriorityColor` / `getTagPillColor` so every TagNote surface speaks the
/// same priority language (`design_docs/ux_guidelines.md` §18). Never hardcoded.
struct PriorityStyle {
    let wash: Color
    let border: Color
    let text: Color
    let label: String
}

enum TagPriority {
    /// Highest importance/urgency among a note's tags, looked up from detailed tag info.
    static func peak(for tags: [String], in available: [TagInfo]) -> (importance: Int, urgency: Int)? {
        var maxI = 0
        var maxU = 0
        var found = false
        for name in tags {
            guard let info = available.first(where: { $0.name == name }) else { continue }
            found = true
            maxI = max(maxI, info.importance)
            maxU = max(maxU, info.urgency)
        }
        return found ? (maxI, maxU) : nil
    }

    /// Card edge + wash + label for the note's highest-priority tag, or nil for neutral.
    static func cardStyle(importance: Int, urgency: Int, isDark: Bool) -> PriorityStyle? {
        guard let core = core(importance: importance, urgency: urgency) else { return nil }
        let sat = (isDark ? 25 + core.dist * 40 : 30 + core.dist * 55) / 100
        let bgLight = (isDark ? 15 + core.dist * 8 : 97 - core.dist * 8) / 100
        let borderLight = (isDark ? 35 + core.dist * 15 : 55 - core.dist * 15) / 100
        let textLight = (isDark ? 70 + core.dist * 10 : 35 - core.dist * 10) / 100
        return PriorityStyle(
            wash: Color(hslHue: core.hue, saturation: sat, lightness: bgLight),
            border: Color(hslHue: core.hue, saturation: sat, lightness: borderLight),
            text: Color(hslHue: core.hue, saturation: sat, lightness: textLight),
            label: core.label
        )
    }

    /// Per-tag pill color for chips colored by priority, or nil for neutral.
    static func pillStyle(importance: Int, urgency: Int, isActive: Bool, isDark: Bool) -> (background: Color, text: Color)? {
        guard let core = core(importance: importance, urgency: urgency) else { return nil }
        let sat: Double
        let bgL: Double
        let textL: Double
        if isActive {
            sat = isDark ? 40 + core.dist * 30 : 50 + core.dist * 35
            bgL = isDark ? 25 + core.dist * 10 : 85 - core.dist * 15
            textL = isDark ? 90 : 15
        } else {
            sat = isDark ? 15 + core.dist * 20 : 20 + core.dist * 30
            bgL = isDark ? 18 + core.dist * 5 : 93 - core.dist * 5
            textL = isDark ? 75 + core.dist * 10 : 40 - core.dist * 10
        }
        return (
            Color(hslHue: core.hue, saturation: sat / 100, lightness: bgL / 100),
            Color(hslHue: core.hue, saturation: sat / 100, lightness: max(0, textL / 100))
        )
    }

    private struct Core {
        let hue: Double
        let dist: Double
        let label: String
    }

    private static func core(importance: Int, urgency: Int) -> Core? {
        let i = Double(importance) / 100
        let u = Double(urgency) / 100
        let dx = i - 0.5
        let dy = u - 0.5
        let dist = min(1, (dx * dx + dy * dy).squareRoot() / 0.707)
        guard dist >= 0.12 else { return nil }

        // Corner hues: low/low slate-blue, high-I green, high-U amber, high/high red.
        let topHue = lerpAngle(35, 0, i)
        let botHue = lerpAngle(215, 145, i)
        let hue = lerpAngle(botHue, topHue, u)

        let label: String
        if i > 0.6 && u > 0.6 { label = "Critical" }
        else if i > 0.6 { label = "Strategic" }
        else if u > 0.6 { label = "Tactical" }
        else if i < 0.35 && u < 0.35 { label = "Archive" }
        else { label = "Normal" }

        return Core(hue: hue, dist: dist, label: label)
    }

    private static func lerpAngle(_ a: Double, _ b: Double, _ t: Double) -> Double {
        let delta = ((b - a + 540).truncatingRemainder(dividingBy: 360)) - 180
        return (a + delta * t + 360).truncatingRemainder(dividingBy: 360)
    }
}

/// Relative timestamp formatting matching the web client's `formatTime`
/// (`design_docs/ux_guidelines.md` §2): relative within a week, absolute beyond.
enum RelativeTime {
    static func format(_ date: Date, now: Date = Date()) -> String {
        let diff = now.timeIntervalSince(date)
        if diff < 60 { return "Just now" }
        let mins = Int(diff / 60)
        if mins < 60 { return "\(mins)m ago" }
        let hours = Int(diff / 3600)
        if hours < 24 { return "\(hours)h ago" }
        let days = Int(diff / 86400)
        if days < 7 { return "\(days)d ago" }

        let formatter = DateFormatter()
        let calendar = Calendar.current
        if calendar.component(.year, from: date) == calendar.component(.year, from: now) {
            formatter.dateFormat = "MMM d"
        } else {
            formatter.dateFormat = "MMM d, yyyy"
        }
        return formatter.string(from: date)
    }
}
