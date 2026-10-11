import AppKit

/// Givens's icon, drawn in code so a dev build and the bundle share one source: a blue
/// rounded square holding a white checklist card, two steps ticked green and one to go.
/// The app sets it as the Dock and ⌘Tab icon at launch; scripts/build-app.sh runs
/// `Givens --write-icon <file.png>` and turns the PNG into Contents/Resources/Givens.icns.
/// Ref: https://developer.apple.com/design/human-interface-guidelines/app-icons#macOS
enum AppIcon {
    /// The icon at a square size in points.
    /// Ref: https://developer.apple.com/documentation/appkit/nsimage/init(size:flipped:drawinghandler:)
    static func image(size: CGFloat = 1024) -> NSImage {
        precondition(size > 0, "an icon has a size")
        return NSImage(size: NSSize(width: size, height: size), flipped: true) { rect in
            draw(in: rect)
            return true
        }
    }

    /// Writes the 1024-point icon as a PNG, for build-app.sh to make the .icns from.
    /// Ref: https://developer.apple.com/documentation/appkit/nsbitmapimagerep
    static func writePNG(to path: String) throws {
        precondition(!path.isEmpty, "the icon goes to a file")
        let side = 1024
        guard
            let bitmap = NSBitmapImageRep(
                bitmapDataPlanes: nil, pixelsWide: side, pixelsHigh: side, bitsPerSample: 8,
                samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB,
                bytesPerRow: 0, bitsPerPixel: 0)
        else { throw CocoaError(.fileWriteUnknown) }
        NSGraphicsContext.saveGraphicsState()
        NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: bitmap)
        image().draw(in: NSRect(x: 0, y: 0, width: side, height: side))
        NSGraphicsContext.restoreGraphicsState()
        guard let png = bitmap.representation(using: .png, properties: [:]) else {
            throw CocoaError(.fileWriteUnknown)
        }
        try png.write(to: URL(fileURLWithPath: path))
    }

    /// Draws into a square rect, top-left origin. Sizes follow Apple's 1024 grid: an 824-point
    /// body with a 185-point corner, centred, leaving room for its shadow.
    private static func draw(in rect: NSRect) {
        let unit = rect.width / 1024
        let body = NSRect(x: 100 * unit, y: 90 * unit, width: 824 * unit, height: 824 * unit)
        let squircle = NSBezierPath(roundedRect: body, xRadius: 185 * unit, yRadius: 185 * unit)

        NSGraphicsContext.saveGraphicsState()
        let shadow = NSShadow()
        shadow.shadowColor = NSColor.black.withAlphaComponent(0.3)
        shadow.shadowBlurRadius = 24 * unit
        shadow.shadowOffset = NSSize(width: 0, height: -10 * unit)
        shadow.set()
        NSColor(srgbRed: 0.16, green: 0.36, blue: 0.95, alpha: 1).setFill()
        squircle.fill()
        NSGraphicsContext.restoreGraphicsState()

        // Ref: https://developer.apple.com/documentation/appkit/nsgradient
        NSGradient(
            starting: NSColor(srgbRed: 0.33, green: 0.55, blue: 1.0, alpha: 1),
            ending: NSColor(srgbRed: 0.12, green: 0.24, blue: 0.75, alpha: 1))?
            .draw(in: squircle, angle: 90)

        let card = NSRect(x: 230 * unit, y: 220 * unit, width: 564 * unit, height: 584 * unit)
        NSGraphicsContext.saveGraphicsState()
        let lift = NSShadow()
        lift.shadowColor = NSColor.black.withAlphaComponent(0.25)
        lift.shadowBlurRadius = 30 * unit
        lift.shadowOffset = NSSize(width: 0, height: -12 * unit)
        lift.set()
        NSColor.white.setFill()
        NSBezierPath(roundedRect: card, xRadius: 64 * unit, yRadius: 64 * unit).fill()
        NSGraphicsContext.restoreGraphicsState()

        for row in 0..<3 {
            let top = card.minY + (110 + CGFloat(row) * 150) * unit
            drawRow(top: top, left: card.minX + 80 * unit, unit: unit, ticked: row < 2)
        }
    }

    /// One checklist row: a tick circle, then a bar standing in for the step's words.
    private static func drawRow(top: CGFloat, left: CGFloat, unit: CGFloat, ticked: Bool) {
        let circle = NSRect(x: left, y: top, width: 84 * unit, height: 84 * unit)
        let green = NSColor(srgbRed: 0.18, green: 0.72, blue: 0.38, alpha: 1)
        let grey = NSColor(srgbRed: 0.78, green: 0.81, blue: 0.86, alpha: 1)
        if ticked {
            green.setFill()
            NSBezierPath(ovalIn: circle).fill()
            let tick = NSBezierPath()
            tick.move(to: NSPoint(x: circle.minX + 22 * unit, y: circle.midY + 2 * unit))
            tick.line(to: NSPoint(x: circle.minX + 37 * unit, y: circle.midY + 17 * unit))
            tick.line(to: NSPoint(x: circle.minX + 63 * unit, y: circle.midY - 14 * unit))
            tick.lineWidth = 11 * unit
            tick.lineCapStyle = .round
            tick.lineJoinStyle = .round
            NSColor.white.setStroke()
            tick.stroke()
        } else {
            let ring = NSBezierPath(ovalIn: circle.insetBy(dx: 6 * unit, dy: 6 * unit))
            ring.lineWidth = 12 * unit
            grey.setStroke()
            ring.stroke()
        }
        let width: CGFloat = ticked ? 300 : 230
        let bar = NSRect(
            x: circle.maxX + 40 * unit, y: circle.midY - 18 * unit, width: width * unit,
            height: 36 * unit)
        (ticked ? NSColor(srgbRed: 0.2, green: 0.25, blue: 0.35, alpha: 1) : grey).setFill()
        NSBezierPath(roundedRect: bar, xRadius: 18 * unit, yRadius: 18 * unit).fill()
    }
}
