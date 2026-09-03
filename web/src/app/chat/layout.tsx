import { SiteHeader } from "@/components/site-header";

export default function ChatLayout({ children }: LayoutProps<"/chat">) {
  return (
    <div className="flex min-h-full flex-1 flex-col">
      <SiteHeader />
      <main className="flex flex-1 flex-col">{children}</main>
    </div>
  );
}
