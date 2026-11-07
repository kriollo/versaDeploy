// Rendered on every request on the server (SSR), never prerendered
export const dynamic = 'force-dynamic';

export default function Page() {
    return <p>versa-e2e next ssr {process.version}</p>;
}
