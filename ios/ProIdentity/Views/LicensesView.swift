import SwiftUI

/// Third-party software in the app and its licenses (the notices MIT and
/// BSD require to ship with the app), plus this app's own license.
struct LicensesView: View {
    struct Component: Identifiable {
        let name: String
        let detail: String
        let license: String
        let file: String
        var id: String { name }
    }

    static let components: [Component] = [
        Component(name: "WireGuardKit", detail: "WireGuard for iOS (wireguard-apple)", license: "MIT", file: "wireguard-apple"),
        Component(name: "wireguard-go", detail: "WireGuard implementation", license: "MIT", file: "wireguard-go"),
        Component(name: "Go", detail: "Go runtime, standard library and golang.org/x/crypto, net, sys", license: "BSD-3-Clause", file: "go"),
    ]

    static let app = Component(name: "ProIdentity Access", detail: "This app", license: "Free Internal Use License 1.0", file: "proidentity-access")

    var body: some View {
        List {
            Section {
                ForEach(Self.components) { row($0) }
            } footer: {
                Text("WireGuard is a registered trademark of Jason A. Donenfeld.")
            }
            .listRowBackground(Brand.surface)

            Section {
                row(Self.app)
                Link(destination: URL(string: "https://github.com/Pro-IT-Services/ProIdentity-Access")!) {
                    LabeledContent("Source code") {
                        Image(systemName: "arrow.up.right.square").foregroundStyle(.secondary)
                    }
                }
            }
            .listRowBackground(Brand.surface)
        }
        .scrollContentBackground(.hidden)
        .background(Brand.background.ignoresSafeArea())
        .navigationTitle("Open-source licenses")
        .navigationBarTitleDisplayMode(.inline)
    }

    private func row(_ c: Component) -> some View {
        NavigationLink {
            LicenseTextView(title: c.name, file: c.file)
        } label: {
            VStack(alignment: .leading, spacing: 2) {
                Text(c.name)
                Text("\(c.detail) · \(c.license)")
                    .font(.footnote)
                    .foregroundStyle(.secondary)
            }
        }
    }
}

private struct LicenseTextView: View {
    let title: String
    let file: String

    private var text: String {
        guard let url = Bundle.main.url(forResource: file, withExtension: "txt"),
              let text = try? String(contentsOf: url, encoding: .utf8) else {
            return "License text not found."
        }
        return text
    }

    var body: some View {
        ScrollView {
            Text(text)
                .font(.system(.footnote, design: .monospaced))
                .textSelection(.enabled)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(16)
        }
        .background(Brand.background.ignoresSafeArea())
        .navigationTitle(title)
        .navigationBarTitleDisplayMode(.inline)
    }
}
